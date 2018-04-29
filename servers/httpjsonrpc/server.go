package httpjsonrpc

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	. "github.com/elastos/Elastos.ELA.SideChain/config"
	"github.com/elastos/Elastos.ELA.SideChain/errors"
	"github.com/elastos/Elastos.ELA.SideChain/log"
	. "github.com/elastos/Elastos.ELA.SideChain/servers"
)

//an instance of the multiplexer
var mainMux map[string]func(Params) map[string]interface{}

const (
	// JSON-RPC protocol error codes.
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
	//-32000 to -32099	Server error, waiting for defining
)

func StartRPCServer() {
	mainMux = make(map[string]func(Params) map[string]interface{})

	http.HandleFunc("/", Handle)

	mainMux["setloglevel"] = SetLogLevel
	mainMux["getinfo"] = GetInfo
	mainMux["getblock"] = GetBlockByHash
	mainMux["getcurrentheight"] = GetCurrentHeight
	mainMux["getblockhash"] = GetBlockHash
	mainMux["getconnectioncount"] = GetConnectionCount
	mainMux["getrawmempool"] = GetTransactionPool
	mainMux["getrawtransaction"] = GetRawTransaction
	mainMux["getneighbors"] = GetNeighbors
	mainMux["getnodestate"] = GetNodeState
	mainMux["sendtransactioninfo"] = SendTransactionInfo
	mainMux["sendrawtransaction"] = SendRawTransaction
	mainMux["submitblock"] = SubmitBlock
	mainMux["getbestblockhash"] = GetBestBlockHash
	mainMux["getblockcount"] = GetBlockCount
	mainMux["gettransaction"] = GetTransactionByHash
	mainMux["getdestroyedtransactions"] = GetDestroyedTransactionsByHeight
	mainMux["getblockbyheight"] = GetBlockByHeight

	// aux interfaces
	mainMux["help"] = AuxHelp
	mainMux["submitauxblock"] = SubmitAuxBlock
	mainMux["createauxblock"] = CreateAuxBlock
	// mining interfaces
	mainMux["togglemining"] = ToggleMining
	mainMux["manualmining"] = ManualMining

	err := http.ListenAndServe("127.0.0.1:"+strconv.Itoa(Parameters.HttpJsonPort), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}

//this is the funciton that should be called in order to answer an rpc call
//should be registered like "http.AddMethod("/", httpjsonrpc.Handle)"
func Handle(w http.ResponseWriter, r *http.Request) {
	//JSON RPC commands should be POSTs
	if r.Method != "POST" {
		log.Warn("HTTP JSON RPC Handle - Method!=\"POST\"")
		http.Error(w, "JSON RPC procotol only allows POST method", http.StatusMethodNotAllowed)
		return
	}

	if r.Header["Content-Type"][0] != "application/json" {
		http.Error(w, "need content type to be application/json", http.StatusUnsupportedMediaType)
		return
	}

	//read the body of the request
	body, _ := ioutil.ReadAll(r.Body)
	request := make(map[string]interface{})
	error := json.Unmarshal(body, &request)
	if error != nil {
		log.Error("HTTP JSON RPC Handle - json.Unmarshal: ", error)
		RPCError(w, http.StatusBadRequest, ParseError, "rpc json parse error:"+error.Error())
		return
	}
	//get the corresponding function
	requestMethod, ok := request["method"].(string)
	if !ok {
		RPCError(w, http.StatusBadRequest, InvalidRequest, "need a method!")
		return
	}
	method, ok := mainMux[requestMethod]
	if !ok {
		RPCError(w, http.StatusNotFound, MethodNotFound, "method "+requestMethod+" not found")
		return
	}

	requestParams := request["params"]
	//Json rpc 1.0 support positional parameters while json rpc 2.0 support named parameters.
	// positional parameters: { "requestParams":[1, 2, 3....] }
	// named parameters: { "requestParams":{ "a":1, "b":2, "c":3 } }
	//Here we support both of them, because bitcion does so.
	var params Params
	switch requestParams := requestParams.(type) {
	case nil:
	case []interface{}:
		params = convertParams(requestMethod, requestParams)
	case map[string]interface{}:
		params = Params(requestParams)
	default:
		RPCError(w, http.StatusBadRequest, InvalidRequest, "params format error, must be an array or a map")
		return
	}

	response := method(params)
	var data []byte
	if response["Error"] != errors.ErrCode(0) {
		data, _ = json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"error": map[string]interface{}{
				"code":    response["Error"],
				"message": response["Result"],
				"id":      request["id"],
			},
		})

	} else {
		data, _ = json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"result":  response["Result"],
			"id":      request["id"],
		})
	}
	w.Write(data)
}

func RPCError(w http.ResponseWriter, httpStatus int, code errors.ErrCode, message string) {
	w.WriteHeader(httpStatus)
	data, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
			"id":      nil,
		},
	})
	w.Write(data)
}

func convertParams(method string, params []interface{}) Params {
	switch method {
	case "createauxblock":
		return FromArray(params, "paytoaddress")
	case "submitauxblock":
		return FromArray(params, "blockhash", "auxpow")
	case "getblockhash":
		return FromArray(params, "index")
	case "getblock":
		return FromArray(params, "hash", "format")
	case "setloglevel":
		return FromArray(params, "level")
	case "getrawtransaction":
		return FromArray(params, "hash", "decoded")
	case "getarbitratorgroupbyheight":
		return FromArray(params, "height")
	case "togglemining":
		return FromArray(params, "mine")
	case "manualmining":
		return FromArray(params, "count")
	default:
		return Params{}
	}
}