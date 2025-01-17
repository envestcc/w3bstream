package wasmtime

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"golang.org/x/text/encoding/unicode"

	conflog "github.com/machinefi/w3bstream/pkg/depends/conf/log"
	confmqtt "github.com/machinefi/w3bstream/pkg/depends/conf/mqtt"
	"github.com/machinefi/w3bstream/pkg/depends/x/mapx"
	"github.com/machinefi/w3bstream/pkg/modules/job"
	"github.com/machinefi/w3bstream/pkg/modules/metrics"
	optypes "github.com/machinefi/w3bstream/pkg/modules/operator/pool/types"
	wasmapi "github.com/machinefi/w3bstream/pkg/modules/vm/wasmapi/types"
	"github.com/machinefi/w3bstream/pkg/types"
	"github.com/machinefi/w3bstream/pkg/types/wasm"
	"github.com/machinefi/w3bstream/pkg/types/wasm/sql_util"
)

type (
	Import func(module, name string, f interface{}) error

	ABILinker interface {
		LinkABI(Import) error
	}

	ExportFuncs struct {
		rt      *Runtime
		res     *mapx.Map[uint32, []byte]
		evs     *mapx.Map[uint32, []byte]
		env     *wasm.Env
		kvs     wasm.KVStore
		db      *wasm.Database
		log     conflog.Logger
		cl      *wasm.ChainClient
		cf      *types.ChainConfig
		ctx     context.Context
		mq      *confmqtt.Client
		metrics metrics.CustomMetrics
		srv     wasmapi.Server
		opPool  optypes.Pool
	}
)

func NewExportFuncs(ctx context.Context, rt *Runtime) (*ExportFuncs, error) {
	ef := &ExportFuncs{
		res:     wasm.MustRuntimeResourceFromContext(ctx),
		evs:     wasm.MustRuntimeEventTypesFromContext(ctx),
		kvs:     wasm.MustKVStoreFromContext(ctx),
		log:     wasm.MustLoggerFromContext(ctx),
		srv:     types.MustWasmApiServerFromContext(ctx),
		opPool:  types.MustOperatorPoolFromContext(ctx),
		cl:      wasm.MustChainClientFromContext(ctx),
		cf:      types.MustChainConfigFromContext(ctx),
		db:      wasm.MustSQLStoreFromContext(ctx),
		env:     wasm.MustEnvFromContext(ctx),
		mq:      wasm.MustMQTTClientFromContext(ctx),
		metrics: wasm.MustCustomMetricsFromContext(ctx),
		rt:      rt,
		ctx:     ctx,
	}

	return ef, nil
}

var (
	_       wasm.ABI = (*ExportFuncs)(nil)
	_rand            = rand.New(rand.NewSource(time.Now().UnixNano()))
	efSrc            = "wasmExportFunc"
	codeSrc          = "wasmCode"
)

func (ef *ExportFuncs) LinkABI(impt Import) error {
	for name, ff := range map[string]interface{}{
		"abort":                    ef.Abort,
		"trace":                    ef.Trace,
		"seed":                     ef.Seed,
		"ws_log":                   ef.Log,
		"ws_get_data":              ef.GetData,
		"ws_set_data":              ef.SetData,
		"ws_get_db":                ef.GetDB,
		"ws_set_db":                ef.SetDB,
		"ws_send_tx":               ef.SendTX,
		"ws_send_tx_with_operator": ef.SendTXWithOperator,
		"ws_call_contract":         ef.CallContract,
		"ws_set_sql_db":            ef.SetSQLDB,
		"ws_get_sql_db":            ef.GetSQLDB,
		"ws_get_env":               ef.GetEnv,
		"ws_send_mqtt_msg":         ef.SendMqttMsg,
		"ws_api_call":              ef.ApiCall,
	} {
		if err := impt("env", name, ff); err != nil {
			return err
		}
	}

	for name, ff := range map[string]interface{}{
		"ws_submit_metrics": ef.StatSubmit,
	} {
		if err := impt("stat", name, ff); err != nil {
			return err
		}
	}

	return nil
}

func (ef *ExportFuncs) logAndPersistToDB(logLevel conflog.Level, logSrc, msg string) {
	ef.log.Debug(fmt.Sprintf("start invoke logAndPersistToDB with %s and %s", logLevel.String(), msg))
	if len(logSrc) == 0 {
		logSrc = efSrc
	}
	ef.log = ef.log.WithValues("@src", logSrc)
	switch logLevel {
	case conflog.TraceLevel:
		ef.log.Trace(msg)
	case conflog.DebugLevel:
		ef.log.Debug(msg)
	case conflog.InfoLevel:
		ef.log.Info(msg)
	case conflog.WarnLevel:
		ef.log.Warn(errors.New(msg))
	case conflog.ErrorLevel:
		ef.log.Error(errors.New(msg))
	default:
		ef.log.Trace(msg)
	}
	job.Dispatch(ef.ctx, job.NewWasmLogTask(ef.ctx, logLevel.String(), logSrc, msg))
}

func (ef *ExportFuncs) Log(logLevel, ptr, size int32) int32 {
	ef.log.Debug("start invoke log")
	buf, err := ef.rt.Read(ptr, size)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, codeSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	ef.logAndPersistToDB(conflog.Level(logLevel), codeSrc, string(buf))
	return int32(wasm.ResultStatusCode_OK)
}

func (ef *ExportFuncs) ApiCall(kAddr, kSize, vmAddrPtr, vmSizePtr int32) int32 {
	buf, err := ef.rt.Read(kAddr, kSize)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_TransDataFromVMFailed)
	}

	resp := ef.srv.Call(ef.ctx, buf)

	respJson, err := json.Marshal(resp)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_HostInternal)
	}

	if err := ef.rt.Copy(respJson, vmAddrPtr, vmSizePtr); err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_TransDataToVMFailed)
	}

	return int32(wasm.ResultStatusCode_OK)
}

// Abort is reserved for imported func env.abort() which is auto-generated by assemblyScript
func (ef *ExportFuncs) Abort(msgPtr int32, fileNamePtr int32, line int32, col int32) {
	msg, err := ef.readString(msgPtr)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, errors.Wrap(err, "fail to decode arguments in env.abort").Error())
		return
	}
	fileName, err := ef.readString(fileNamePtr)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, errors.Wrap(err, "fail to decode arguments in env.abort").Error())
		return
	}
	ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, errors.Errorf("abort: %s at %s:%d:%d", msg, fileName, line, col).Error())
}

func (ef *ExportFuncs) readString(ptr int32) (string, error) {
	if ptr < 4 {
		return "", errors.Errorf("the pointer address %d is invalid", ptr)
	}

	decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()

	lenData, err := ef.rt.Read(ptr-4, 4) // sizeof(uint32) is 4
	if err != nil {
		return "", err
	}
	len := binary.LittleEndian.Uint32(lenData)
	data, err := ef.rt.Read(ptr, int32(len))
	if err != nil {
		return "", err
	}
	utf8bytes, err := decoder.Bytes(data)
	if err != nil {
		return "", err
	}
	return string(utf8bytes), nil
}

// Trace is reserved for imported func env.trace() which is auto-generated by assemblyScript
func (ef *ExportFuncs) Trace(msgPtr int32, _ int32, arr ...float64) {
	msg, err := ef.readString(msgPtr)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, errors.Wrap(err, "fail to decode arguments in env.abort").Error())
		return
	}

	str := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(arr)), ", "), "[]")
	if len(str) > 0 {
		str = " " + str
	}
	ef.logAndPersistToDB(conflog.InfoLevel, efSrc, fmt.Sprintf("trace: %s%s", msg, str))
}

// Seed is reserved for imported func env.seed() which is auto-generated by assemblyScript
func (ef *ExportFuncs) Seed() float64 {
	return _rand.Float64() * float64(time.Now().UnixNano())
}

func (ef *ExportFuncs) GetData(rid, vmAddrPtr, vmSizePtr int32) int32 {
	data, ok := ef.res.Load(uint32(rid))
	if !ok {
		return int32(wasm.ResultStatusCode_ResourceNotFound)
	}

	if err := ef.rt.Copy(data, vmAddrPtr, vmSizePtr); err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_TransDataToVMFailed)
	}

	return int32(wasm.ResultStatusCode_OK)
}

// TODO SetData if rid not exist, should be assigned by wasm?
func (ef *ExportFuncs) SetData(rid, addr, size int32) int32 {
	buf, err := ef.rt.Read(addr, size)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_TransDataToVMFailed)
	}
	ef.res.Store(uint32(rid), buf)
	return int32(wasm.ResultStatusCode_OK)
}

func (ef *ExportFuncs) GetDB(kAddr, kSize int32, vmAddrPtr, vmSizePtr int32) int32 {
	key, err := ef.rt.Read(kAddr, kSize)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_ResourceNotFound)
	}

	val, exist := ef.kvs.Get(string(key))
	if exist != nil || val == nil {
		return int32(wasm.ResultStatusCode_ResourceNotFound)
	}

	ef.logAndPersistToDB(conflog.InfoLevel, efSrc, fmt.Sprintf("host.GetDB %s:%s", string(key), strconv.Quote(string(val))))

	if err := ef.rt.Copy(val, vmAddrPtr, vmSizePtr); err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_TransDataToVMFailed)
	}

	return int32(wasm.ResultStatusCode_OK)
}

func (ef *ExportFuncs) SetDB(kAddr, kSize, vAddr, vSize int32) int32 {
	key, err := ef.rt.Read(kAddr, kSize)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_ResourceNotFound)
	}
	val, err := ef.rt.Read(vAddr, vSize)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_ResourceNotFound)
	}

	ef.logAndPersistToDB(conflog.InfoLevel, efSrc, fmt.Sprintf("host.SetDB %s:%s", string(key), strconv.Quote(string(val))))

	err = ef.kvs.Set(string(key), val)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_Failed)
	}
	return int32(wasm.ResultStatusCode_OK)
}

func (ef *ExportFuncs) SetSQLDB(addr, size int32) int32 {
	if ef.db == nil {
		return int32(wasm.ResultStatusCode_NoDBContext)
	}
	data, err := ef.rt.Read(addr, size)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_ResourceNotFound)
	}

	prestate, params, err := sql_util.ParseQuery(data)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}

	db, err := ef.db.WithDefaultSchema()
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	_, err = db.ExecContext(context.Background(), prestate, params...)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}

	return int32(wasm.ResultStatusCode_OK)
}

func (ef *ExportFuncs) GetSQLDB(addr, size int32, vmAddrPtr, vmSizePtr int32) int32 {
	if ef.db == nil {
		return int32(wasm.ResultStatusCode_NoDBContext)
	}
	data, err := ef.rt.Read(addr, size)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_ResourceNotFound)
	}

	prestate, params, err := sql_util.ParseQuery(data)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}

	db, err := ef.db.WithDefaultSchema()
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	rows, err := db.QueryContext(context.Background(), prestate, params...)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}

	ret, err := sql_util.JsonifyRows(rows)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}

	if err := ef.rt.Copy(ret, vmAddrPtr, vmSizePtr); err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_TransDataToVMFailed)
	}

	return int32(wasm.ResultStatusCode_OK)
}

// TODO: make sendTX async, and add callback if possible
func (ef *ExportFuncs) SendTX(chainID int32, offset, size, vmAddrPtr, vmSizePtr int32) int32 {
	if ef.cl == nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, errors.New("eth client doesn't exist").Error())
		return wasm.ResultStatusCode_Failed
	}
	buf, err := ef.rt.Read(offset, size)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	ret := gjson.Parse(string(buf))
	txHash, err := ef.cl.SendTX(ef.cf, uint64(chainID), "", ret.Get("to").String(), ret.Get("value").String(), ret.Get("data").String(), ef.opPool, types.MustProjectFromContext(ef.ctx))
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	if err := ef.rt.Copy([]byte(txHash), vmAddrPtr, vmSizePtr); err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	return int32(wasm.ResultStatusCode_OK)
}

func (ef *ExportFuncs) SendTXWithOperator(chainID int32, offset, size, vmAddrPtr, vmSizePtr int32) int32 {
	if ef.cl == nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, errors.New("eth client doesn't exist").Error())
		return wasm.ResultStatusCode_Failed
	}
	buf, err := ef.rt.Read(offset, size)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	ret := gjson.Parse(string(buf))
	txHash, err := ef.cl.SendTXWithOperator(ef.cf, uint64(chainID), "", ret.Get("to").String(), ret.Get("value").String(), ret.Get("data").String(), ret.Get("operatorName").String(), ef.opPool, types.MustProjectFromContext(ef.ctx))
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	if err := ef.rt.Copy([]byte(txHash), vmAddrPtr, vmSizePtr); err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	return int32(wasm.ResultStatusCode_OK)
}

func (ef *ExportFuncs) SendMqttMsg(topicAddr, topicSize, msgAddr, msgSize int32) int32 {
	if ef.mq == nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, errors.New("mq client doesn't exist").Error())
		return wasm.ResultStatusCode_Failed
	}

	var (
		topicBuf []byte
		msgBuf   []byte
		err      error
	)

	topicBuf, err = ef.rt.Read(topicAddr, topicSize)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	msgBuf, err = ef.rt.Read(msgAddr, msgSize)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	err = ef.mq.WithTopic(string(topicBuf)).Publish(string(msgBuf))
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	return int32(wasm.ResultStatusCode_OK)
}

func (ef *ExportFuncs) CallContract(chainID int32, offset, size int32, vmAddrPtr, vmSizePtr int32) int32 {
	if ef.cl == nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, errors.New("eth client doesn't exist").Error())
		return wasm.ResultStatusCode_Failed
	}
	buf, err := ef.rt.Read(offset, size)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	ret := gjson.Parse(string(buf))
	data, err := ef.cl.CallContract(ef.cf, uint64(chainID), "", ret.Get("to").String(), ret.Get("data").String())
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	if err = ef.rt.Copy(data, vmAddrPtr, vmSizePtr); err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	return int32(wasm.ResultStatusCode_OK)
}

func (ef *ExportFuncs) GetEnv(kAddr, kSize int32, vmAddrPtr, vmSizePtr int32) int32 {
	if ef.env == nil {
		return int32(wasm.ResultStatusCode_EnvKeyNotFound)
	}
	key, err := ef.rt.Read(kAddr, kSize)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_TransDataToVMFailed)
	}

	val, ok := ef.env.Get(string(key))
	if !ok {
		return int32(wasm.ResultStatusCode_EnvKeyNotFound)
	}

	if err = ef.rt.Copy([]byte(val), vmAddrPtr, vmSizePtr); err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_TransDataToVMFailed)
	}
	return int32(wasm.ResultStatusCode_OK)
}

func (ef *ExportFuncs) GetEventType(rid, vmAddrPtr, vmSizePtr int32) int32 {
	data, ok := ef.res.Load(uint32(rid))
	if !ok {
		return int32(wasm.ResultStatusCode_ResourceNotFound)
	}

	if err := ef.rt.Copy(data, vmAddrPtr, vmSizePtr); err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return int32(wasm.ResultStatusCode_TransDataToVMFailed)
	}
	return int32(wasm.ResultStatusCode_OK)
}

func (ef *ExportFuncs) StatSubmit(vmAddrPtr, vmSizePtr int32) int32 {
	buf, err := ef.rt.Read(vmAddrPtr, vmSizePtr)
	if err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	str := string(buf)
	if !gjson.Valid(str) {
		err = errors.New("invalid json")
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	object := gjson.Parse(str)
	if object.IsArray() {
		err = errors.New("json object should not be an array")
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}

	if err := ef.metrics.Submit(object); err != nil {
		ef.logAndPersistToDB(conflog.ErrorLevel, efSrc, err.Error())
		return wasm.ResultStatusCode_Failed
	}
	return int32(wasm.ResultStatusCode_OK)
}
