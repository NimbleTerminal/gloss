package document

import (
	"bytes"
	"compress/zlib"
	"encoding/ascii85"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"time"
)

func budgetStream(filter string, raw []byte) []byte {
	return []byte(fmt.Sprintf("<< /Length %d %s >>\nstream\n%s\nendstream\n", len(raw), filter, raw))
}
func deflateForTest(t *testing.T, data []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func ascii85ForTest(data []byte) []byte {
	var b bytes.Buffer
	w := ascii85.NewEncoder(&b)
	_, _ = w.Write(data)
	_ = w.Close()
	b.WriteString("~>")
	return b.Bytes()
}
func smallPDFBudget(data []byte) error {
	b := &pdfStreamBudget{maxStream: 1024, maxTotal: 4096, until: time.Now().Add(time.Second), integers: map[pdfRef]int64{}}
	return b.scan(data, 0)
}
func TestPDFStreamFilterBudgets(t *testing.T) {
	for _, size := range []int{100, 1024, 1025, 8192} {
		raw := bytes.Repeat([]byte(" "), size)
		compressed := deflateForTest(t, raw)
		for _, tc := range []struct {
			name, filter string
			data         []byte
		}{
			{"plain", "", raw},
			{"flate", "/Filter /FlateDecode", compressed},
			{"ascii85-flate", "/Filter [/ASCII85Decode /FlateDecode]", ascii85ForTest(compressed)},
			{"escaped names", "/Filter [/ASCII85Decode /Fl#61teDecode]", ascii85ForTest(compressed)},
		} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, size), func(t *testing.T) {
				err := smallPDFBudget(budgetStream(tc.filter, tc.data))
				if (err != nil) != (size > 1024) {
					t.Fatalf("size %d: %v", size, err)
				}
			})
		}
	}
}
func TestPDFStreamBoundaries(t *testing.T) {
	// Literal stream markers in data and strings cannot truncate the check.
	payload := []byte(strings.Repeat("endstream ", 150))
	if err := smallPDFBudget(budgetStream("", payload)); err == nil {
		t.Fatal("embedded endstream bypassed length")
	}
	if err := smallPDFBudget([]byte("(stream\nignored endstream)\n")); err != nil {
		t.Fatal(err)
	}
	indirect := strings.Replace(string(budgetStream("", []byte("abc"))), "/Length 3", "/Length 9 0 R", 1) + "\n9 0 obj\n3\nendobj\n"
	if err := checkPDFStreams([]byte(indirect)); err != nil {
		t.Fatal(err)
	}
	if err := checkPDFStreams([]byte(indirect + "9 0 obj\n400\nendobj\n")); err == nil {
		t.Fatal("ambiguous indirect length accepted")
	}
	for _, bad := range []string{
		"<< /Length 100 >>\nstream\na\nendstream",
		"<< /Length 1 /Filter 9 0 R >>\nstream\na\nendstream",
		"<< /Length 1 /Filter /Unknown >>\nstream\na\nendstream",
		"<< /Length 1 /Filter (FlateDecode) >>\nstream\na\nendstream",
		"<< /Length 1 /Filter <466c6174654465636f6465> >>\nstream\na\nendstream",
		"<< /Length 1 /Length 2 >>\nstream\na\nendstream",
		"xref\n0 999999999\n",
		"<< /Type /XRef /Size 999999999 >>",
	} {
		if err := checkPDFStreams([]byte(bad)); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
func TestPDFPredictorAndTotalBudget(t *testing.T) {
	compressed := deflateForTest(t, []byte("small"))
	for _, params := range []string{"/DecodeParms << /Predictor 12 /Columns 2147483647 >>", "/DecodeParms [<< /Columns -1 >>]", "/DecodeParms 9 0 R"} {
		if err := checkPDFStreams(budgetStream("/Filter /FlateDecode "+params, compressed)); err == nil {
			t.Fatal("unsafe predictor accepted")
		}
	}
	data := bytes.Repeat(budgetStream("", bytes.Repeat([]byte("x"), 1000)), 5)
	if err := smallPDFBudget(data); err == nil {
		t.Fatal("aggregate decoded budget bypassed")
	}
}

func FuzzPDFStreamPreflight(f *testing.F) {
	f.Add(samplePDF())
	f.Add(budgetStream("", []byte("endstream")))
	f.Add([]byte("<< /DecodeParms [<<>>] /Length 0 /Filter /FlateDecode >>\nstream\n\nendstream"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		_ = checkPDFStreams(data)
	})
}

func TestPDFHiddenObjectReferenceRejected(t *testing.T) {
	data := pdfFile("1.4", "", "<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Count 0 /Kids [] >>", "(ordinary)", "(4 0 obj\n<< /Length 3 >>\nstream\nabc\nendstream\nendobj)")
	if err := checkPDFStreams(data); err != nil {
		t.Fatal(err)
	}
	real := bytes.Index(data, []byte("4 0 obj"))
	hidden := bytes.LastIndex(data, []byte("4 0 obj"))
	data = bytes.Replace(data, fmt.Appendf(nil, "%010d 00000 n", real), fmt.Appendf(nil, "%010d 00000 n", hidden), 1)
	if err := checkPDFStreams(data); err == nil {
		t.Fatal("xref followed an object hidden inside a string")
	}
}

func TestPDFHiddenCrossReferenceRejected(t *testing.T) {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Count 0 /Kids [] >>", "(xref\n0 1\n0000000000 65535 f\ntrailer\n<< /Size 1 >>)"}
	data := pdfFile("1.4", "", objects...)
	hidden := bytes.Index(data, []byte("xref\n"))
	data = pdfFile("1.4", fmt.Sprintf("/Prev %d", hidden), objects...)
	if err := checkPDFStreams(data); err == nil {
		t.Fatal("Prev followed a hidden xref")
	}
	data = pdfFile("1.4", "", objects...)
	data = append(data, []byte(fmt.Sprintf("(\nstartxref\n%d\n)\n%%%%EOF\n", hidden))...)
	if err := checkPDFStreams(data); err == nil {
		t.Fatal("final startxref inside a string was accepted")
	}
}

func TestPDFCrossReferenceAndObjectStreams(t *testing.T) {
	for _, predictor := range []bool{false, true} {
		t.Run(fmt.Sprint(predictor), func(t *testing.T) {
			var data bytes.Buffer
			data.WriteString("%PDF-1.5\n")
			// Catalog, page tree and page are compressed objects 1..3 inside object 6.
			objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Count 1 /Kids [3 0 R] >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 72 72] /Contents 4 0 R >>"}
			var header, body bytes.Buffer
			for i, obj := range objects {
				fmt.Fprintf(&header, "%d %d ", i+1, body.Len())
				body.WriteString(obj + "\n")
			}
			offsets := map[int]int{}
			offsets[4] = data.Len()
			data.WriteString("4 0 obj\n")
			data.Write(budgetStream("", []byte("q Q\n")))
			data.WriteString("endobj\n")
			offsets[6] = data.Len()
			data.WriteString("6 0 obj\n")
			raw := append(header.Bytes(), body.Bytes()...)
			data.Write(budgetStream(fmt.Sprintf("/Type /ObjStm /N 3 /First %d /Filter /FlateDecode", header.Len()), deflateForTest(t, raw)))
			data.WriteString("endobj\n")
			offsets[5] = data.Len()
			var table bytes.Buffer
			for i := 0; i < 7; i++ {
				var row [7]byte
				switch {
				case i == 0:
					binary.BigEndian.PutUint16(row[5:], 65535)
				case i <= 3:
					row[0] = 2
					binary.BigEndian.PutUint32(row[1:], 6)
					binary.BigEndian.PutUint16(row[5:], uint16(i-1))
				default:
					row[0] = 1
					binary.BigEndian.PutUint32(row[1:], uint32(offsets[i]))
				}
				table.Write(row[:])
			}
			params := ""
			payload := table.Bytes()
			if predictor {
				var predicted bytes.Buffer
				prev := make([]byte, 7)
				for len(payload) > 0 {
					predicted.WriteByte(2)
					for i, c := range payload[:7] {
						predicted.WriteByte(c - prev[i])
						prev[i] = c
					}
					payload = payload[7:]
				}
				payload = predicted.Bytes()
				params = "/DecodeParms << /Predictor 12 /Columns 7 >>"
			}
			data.WriteString("5 0 obj\n")
			data.Write(budgetStream("/Type /XRef /Root 1 0 R /Size 7 /W [1 4 2] /Filter /FlateDecode "+params, deflateForTest(t, payload)))
			data.WriteString("endobj\n")
			fmt.Fprintf(&data, "startxref\n%d\n%%%%EOF\n", offsets[5])
			if err := checkPDFStreams(data.Bytes()); err != nil {
				t.Fatal(err)
			}
			reply, err := runParser(parserRequest{Kind: "pdf", Data: data.Bytes(), Page: 1})
			if err != nil || reply.Pages != 1 {
				t.Fatalf("worker: pages=%d err=%v", reply.Pages, err)
			}
		})
	}
}
