package core

import (
	"devfeel/framework/json"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var GlobalState *ServerStateInfo

const (
	minuteTimeLayout        = "200601021504"
	dateTimeLayout          = "2006-01-02 15:04:05"
	defaultReserveMinutes   = 60
	defaultCheckTimeMinutes = 10
)

func init() {
	GlobalState = &ServerStateInfo{
		ServerStartTime:       time.Now(),
		TotalRequestCount:     0,
		TotalErrorCount:       0,
		IntervalRequestData:   NewItemContext(),
		DetailRequestPageData: NewItemContext(),
		IntervalErrorData:     NewItemContext(),
		DetailErrorPageData:   NewItemContext(),
		DetailErrorData:       NewItemContext(),
		DetailHttpCodeData:    NewItemContext(),
		dataChan_Request:      make(chan *RequestInfo, 1000),
		dataChan_Error:        make(chan *ErrorInfo, 1000),
		dataChan_HttpCode:     make(chan *HttpCodeInfo, 1000),
		infoPool: &pool{
			requestInfo: sync.Pool{
				New: func() interface{} {
					return &RequestInfo{}
				},
			},
			errorInfo: sync.Pool{
				New: func() interface{} {
					return &ErrorInfo{}
				},
			},
			httpCodeInfo: sync.Pool{
				New: func() interface{} {
					return &HttpCodeInfo{}
				},
			},
		},
	}
	go GlobalState.handleInfo()
	go time.AfterFunc(time.Duration(defaultCheckTimeMinutes)*time.Minute, GlobalState.checkAndRemoveIntervalData)
}

//pool定义
type pool struct {
	requestInfo  sync.Pool
	errorInfo    sync.Pool
	httpCodeInfo sync.Pool
}

type RequestInfo struct {
	Url string
	Num uint64
}
type ErrorInfo struct {
	Url    string
	ErrMsg string
	Num    uint64
}

type HttpCodeInfo struct {
	Url  string
	Code int
	Num  uint64
}

//服务器状态信息
type ServerStateInfo struct {
	//服务启动时间
	ServerStartTime time.Time
	//该运行期间总访问次数
	TotalRequestCount uint64
	//单位时间内请求数据 - 按分钟为单位
	IntervalRequestData *ItemContext
	//明细请求页面数据 - 以不带参数的访问url为key
	DetailRequestPageData *ItemContext
	//该运行期间异常次数
	TotalErrorCount uint64
	//单位时间内异常次数 - 按分钟为单位
	IntervalErrorData *ItemContext
	//明细异常页面数据 - 以不带参数的访问url为key
	DetailErrorPageData *ItemContext
	//明细异常数据 - 以不带参数的访问url为key
	DetailErrorData *ItemContext
	//明细Http状态码数据 - 以HttpCode为key，例如200、500等
	DetailHttpCodeData *ItemContext

	dataChan_Request  chan *RequestInfo
	dataChan_Error    chan *ErrorInfo
	dataChan_HttpCode chan *HttpCodeInfo
	//对象池
	infoPool *pool
}

//ShowHtmlData show server state data html-string format
func (state *ServerStateInfo) ShowHtmlData() string {
	data := "<html><body><div>"
	data += "ServerStartTime : " + state.ServerStartTime.Format(dateTimeLayout)
	data += "<br>"
	data += "TotalRequestCount : " + strconv.FormatUint(state.TotalRequestCount, 10)
	data += "<br>"
	data += "TotalErrorCount : " + strconv.FormatUint(state.TotalErrorCount, 10)
	data += "<br>"
	state.IntervalRequestData.RLock()
	data += "IntervalRequestData : " + jsonutil.GetJsonString(state.IntervalRequestData.GetCurrentMap())
	state.IntervalRequestData.RUnlock()
	data += "<br>"
	state.DetailRequestPageData.RLock()
	data += "DetailRequestPageData : " + jsonutil.GetJsonString(state.DetailRequestPageData.GetCurrentMap())
	state.DetailRequestPageData.RUnlock()
	data += "<br>"
	state.IntervalErrorData.RLock()
	data += "IntervalErrorData : " + jsonutil.GetJsonString(state.IntervalErrorData.GetCurrentMap())
	state.IntervalErrorData.RUnlock()
	data += "<br>"
	state.DetailErrorPageData.RLock()
	data += "DetailErrorPageData : " + jsonutil.GetJsonString(state.DetailErrorPageData.GetCurrentMap())
	state.DetailErrorPageData.RUnlock()
	data += "<br>"
	state.DetailErrorData.RLock()
	data += "DetailErrorData : " + jsonutil.GetJsonString(state.DetailErrorData.GetCurrentMap())
	state.DetailErrorData.RUnlock()
	data += "<br>"
	state.DetailHttpCodeData.RLock()
	data += "DetailHttpCodeData : " + jsonutil.GetJsonString(state.DetailHttpCodeData.GetCurrentMap())
	state.DetailHttpCodeData.RUnlock()
	data += "</div></body></html>"
	return data
}

//QueryIntervalRequestData query request count by query time
func (state *ServerStateInfo) QueryIntervalRequestData(queryKey string) uint64 {
	return state.IntervalRequestData.GetUInt64(queryKey)
}

//QueryIntervalErrorData query error count by query time
func (state *ServerStateInfo) QueryIntervalErrorData(queryKey string) uint64 {
	return state.IntervalErrorData.GetUInt64(queryKey)
}

//增加请求数
func (state *ServerStateInfo) AddRequestCount(page string, num uint64) uint64 {
	if strings.Index(page, "/dotweb/") != 0 {
		atomic.AddUint64(&state.TotalRequestCount, num)
		state.addRequestData(page, num)
	}
	return state.TotalRequestCount
}

//增加Http状态码数据
func (state *ServerStateInfo) AddHttpCodeCount(page string, code int, num uint64) uint64 {
	if strings.Index(page, "/dotweb/") != 0 {
		state.addHttpCodeData(page, code, num)
	}
	return state.TotalErrorCount
}

//增加错误数
func (state *ServerStateInfo) AddErrorCount(page string, err error, num uint64) uint64 {
	atomic.AddUint64(&state.TotalErrorCount, num)
	state.addErrorData(page, err, num)
	return state.TotalErrorCount
}

func (state *ServerStateInfo) addRequestData(page string, num uint64) {
	//get from pool
	info := state.infoPool.requestInfo.Get().(*RequestInfo)
	info.Url = page
	info.Num = num
	state.dataChan_Request <- info
}

func (state *ServerStateInfo) addErrorData(page string, err error, num uint64) {
	//get from pool
	info := state.infoPool.errorInfo.Get().(*ErrorInfo)
	info.Url = page
	info.ErrMsg = err.Error()
	info.Num = num
	state.dataChan_Error <- info
}

func (state *ServerStateInfo) addHttpCodeData(page string, code int, num uint64) {
	//get from pool
	info := state.infoPool.httpCodeInfo.Get().(*HttpCodeInfo)
	info.Url = page
	info.Code = code
	info.Num = num
	state.dataChan_HttpCode <- info
}

//处理日志内部函数
func (state *ServerStateInfo) handleInfo() {
	for {
		select {
		case info := <-state.dataChan_Request:
			{
				//set detail page data
				key := strings.ToLower(info.Url)
				val := state.DetailRequestPageData.GetUInt64(key)
				state.DetailRequestPageData.Set(key, val+info.Num)

				//set interval data
				key = time.Now().Format(minuteTimeLayout)
				val = state.IntervalRequestData.GetUInt64(key)
				state.IntervalRequestData.Set(key, val+info.Num)

				//put info obj
				state.infoPool.requestInfo.Put(info)
			}
		case info := <-state.dataChan_Error:
			{
				//set detail error page data
				key := strings.ToLower(info.Url)
				val := state.DetailErrorPageData.GetUInt64(key)
				state.DetailErrorPageData.Set(key, val+info.Num)

				//set detail error data
				key = info.ErrMsg
				val = state.DetailErrorData.GetUInt64(key)
				state.DetailErrorData.Set(key, val+info.Num)

				//set interval data
				key = time.Now().Format(minuteTimeLayout)
				val = state.IntervalErrorData.GetUInt64(key)
				state.IntervalErrorData.Set(key, val+info.Num)

				//put info obj
				state.infoPool.errorInfo.Put(info)
			}
		case info := <-state.dataChan_HttpCode:
			{
				//set detail error page data
				key := strconv.Itoa(info.Code)
				val := state.DetailHttpCodeData.GetUInt64(key)
				state.DetailHttpCodeData.Set(key, val+info.Num)

				//put info obj
				state.infoPool.httpCodeInfo.Put(info)
			}
		}
	}
}

//check and remove need to remove interval data with request and error
func (state *ServerStateInfo) checkAndRemoveIntervalData() {
	var needRemoveKey []string

	//check IntervalRequestData
	state.IntervalRequestData.RLock()
	if state.IntervalRequestData.Len() > 10 {
		for k, _ := range state.IntervalRequestData.GetCurrentMap() {
			if t, err := time.Parse(minuteTimeLayout, k); err != nil {
				needRemoveKey = append(needRemoveKey, k)
			} else {
				if time.Now().Sub(t) > defaultReserveMinutes {
					needRemoveKey = append(needRemoveKey, k)
				}
			}
		}
	}
	state.IntervalRequestData.RUnlock()
	//remove keys
	for _, v := range needRemoveKey {
		state.IntervalRequestData.Remove(v)
	}

	//check IntervalErrorData
	needRemoveKey = []string{}
	state.IntervalErrorData.RLock()
	if state.IntervalErrorData.Len() > 10 {
		for k, _ := range state.IntervalErrorData.GetCurrentMap() {
			if t, err := time.Parse(minuteTimeLayout, k); err != nil {
				needRemoveKey = append(needRemoveKey, k)
			} else {
				if time.Now().Sub(t) > defaultReserveMinutes {
					needRemoveKey = append(needRemoveKey, k)
				}
			}
		}
	}
	//remove keys
	for _, v := range needRemoveKey {
		state.IntervalErrorData.Remove(v)
	}

	time.AfterFunc(time.Duration(defaultCheckTimeMinutes)*time.Minute, state.checkAndRemoveIntervalData)
}
