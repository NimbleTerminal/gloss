//go:build js && wasm

package document

import (
	"encoding/json"
	"errors"
	"os"
	"syscall/js"
)

func init() {
	if len(os.Args) == 2 && os.Args[1] == parserWorkerArg {
		var q parserRequest
		input := js.Global().Get("glossParserInput").String()
		var out parserReply
		if len(input) > maxParserMessage {
			out.Error = "parser input exceeds limit"
		} else if err := json.Unmarshal([]byte(input), &q); err != nil {
			out.Error = err.Error()
		} else {
			out = parserWork(q)
		}
		encoded, err := json.Marshal(out)
		if err != nil {
			js.Global().Call("postMessage", `{"Error":"could not encode parser reply"}`)
		} else {
			js.Global().Call("postMessage", string(encoded))
		}
		os.Exit(0)
	}
}

func runParser(q parserRequest) (parserReply, error) {
	start := js.Global().Get("glossParseWorker")
	if start.Type() != js.TypeFunction {
		return parserReply{}, errors.New("isolated parser worker is unavailable")
	}
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
	start.Invoke(string(input), parseDeadline.Milliseconds()).Call("then", yes, no)
	reply := <-done
	if reply.err != nil {
		return parserReply{}, reply.err
	}
	if len(reply.value) > maxParserMessage {
		return parserReply{}, errors.New("parser reply exceeds limit")
	}
	var out parserReply
	if err := json.Unmarshal([]byte(reply.value), &out); err != nil {
		return out, err
	}
	if out.Error != "" {
		return out, errors.New(out.Error)
	}
	return out, nil
}
