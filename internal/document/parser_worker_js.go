//go:build js && wasm

package document

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall/js"
)

// In a Web Worker, gloss answers the request it was started with. For a PDF
// that opens, it then stays, answering what the page posts to
// glossParserNext, until the page terminates the worker.
func init() {
	if len(os.Args) == 2 && os.Args[1] == parserWorkerArg {
		var s parserServer
		post := func(input string) bool {
			var q parserRequest
			var out parserReply
			if len(input) > maxParserMessage {
				out = parserReply{Error: "parser input exceeds limit", Ended: true}
			} else if err := json.Unmarshal([]byte(input), &q); err != nil {
				out = parserReply{Error: err.Error(), Ended: s.doc == nil}
			} else {
				out = s.answer(q)
			}
			encoded, err := json.Marshal(out)
			if err != nil {
				encoded = []byte(`{"Error":"could not encode parser reply","Ended":true}`)
				out.Ended = true
			}
			js.Global().Call("postMessage", string(encoded))
			return !out.Ended
		}
		if !post(js.Global().Get("glossParserInput").String()) {
			os.Exit(0)
		}
		js.Global().Set("glossParserNext", js.FuncOf(func(_ js.Value, a []js.Value) any {
			input := a[0].String()
			go func() {
				if !post(input) {
					os.Exit(0)
				}
			}()
			return nil
		}))
		select {}
	}
}

// A browser worker is a session of the page's parser workers.
type pageWorker struct {
	session js.Value
	once    sync.Once
}

func dialParser() (parserConn, error) {
	workers := js.Global().Get("glossParseWorker")
	if workers.Type() != js.TypeObject || workers.Get("session").Type() != js.TypeFunction {
		return nil, errors.New("isolated parser worker is unavailable")
	}
	return &pageWorker{session: workers.Call("session")}, nil
}

func (w *pageWorker) ask(ctx context.Context, q parserRequest) (parserReply, error) {
	input, err := json.Marshal(q)
	if err != nil {
		return parserReply{}, err
	}
	type result struct {
		value string
		err   error
	}
	done := make(chan result, 1)
	yes := js.FuncOf(func(_ js.Value, a []js.Value) any { done <- result{value: a[0].String()}; return nil })
	no := js.FuncOf(func(_ js.Value, a []js.Value) any { done <- result{err: errors.New(a[0].String())}; return nil })
	defer yes.Release()
	defer no.Release()
	w.session.Call("ask", string(input), parseDeadline.Milliseconds()).Call("then", yes, no)
	var reply result
	select {
	case reply = <-done:
	case <-ctx.Done():
		w.close()
		<-done // Closing rejects the question.
		return parserReply{}, fmt.Errorf("%s parsing stopped: %w", q.Kind, ctx.Err())
	}
	if reply.err != nil {
		w.close()
		return parserReply{}, reply.err
	}
	if len(reply.value) > maxParserMessage {
		w.close()
		return parserReply{}, errors.New("parser reply exceeds limit")
	}
	var out parserReply
	if err := json.Unmarshal([]byte(reply.value), &out); err != nil {
		w.close()
		return out, err
	}
	if out.Error != "" {
		return out, errors.New(out.Error)
	}
	return out, nil
}

func (w *pageWorker) close() {
	w.once.Do(func() { w.session.Call("close") })
}
