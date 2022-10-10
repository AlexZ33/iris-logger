package iris_logger

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"github.com/kataras/iris/v12"
	irisContext "github.com/kataras/iris/v12/context"
	"github.com/pelletier/go-toml"
	rpcclient "github.com/smallnest/rpcx/client"
	"github.com/smallnest/rpcx/share"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

//func GetJSON(value interface{}) []byte {
//
//}

func GetBool(tree *toml.Tree, key string, values ...bool) bool {
	value := tree.Get(key)
	if value != nil {
		switch value.(type) {
		case bool:
			return value.(bool)
		case string:
			value, err := strconv.ParseBool(value.(string))
			if err != nil {
				log.Println(err)
			} else {
				return value
			}
		}
	}
	if len(values) > 0 {
		return values[0]
	}
	return false
}

func GetTree(tree *toml.Tree, key string) *toml.Tree {
	if value, ok := tree.Get(key).(*toml.Tree); ok {
		return value
	}
	return new(toml.Tree)
}

func GetString(tree *toml.Tree, key string, values ...string) string {
	value := tree.Get(key)
	if value != nil {
		return ParseString(value)
	} else if len(values) > 0 {
		return values[0]
	}
	return ""
}

func GetStringArray(tree *toml.Tree, key string, values ...[]string) []string {
	strings := make([]string, 0)
	if array, ok := tree.Get(key).([]interface{}); ok {
		for _, value := range array {
			strings = append(strings, ParseString(value))
		}
	}
	if len(strings) == 0 && len(values) > 0 {
		return values[0]
	}
	return strings
}

func ParseString(value interface{}, values ...string) string {
	switch value.(type) {
	case string:
		return value.(string)
	case int64:
		return strconv.FormatInt(value.(int64), 10)
	case uint64:
		return strconv.FormatUint(value.(uint64), 10)
	case float64:
		return strconv.FormatFloat(value.(float64), 'f', -1, 64)
	case bool:
		return strconv.FormatBool(value.(bool))
	case []string:
		return strings.Join(value.([]string), ",")
	case []byte:
		return string(value.([]byte))
	case time.Time:
		return StringifyTime(value.(time.Time))
	case []int64:
		numbers := make([]string, 0)
		for _, number := range value.([]int64) {
			numbers = append(numbers, strconv.FormatInt(number, 10))
		}
		return strings.Join(numbers, ",")
	case []uint64:
		numbers := make([]string, 0)
		for _, number := range value.([]uint64) {
			numbers = append(numbers, strconv.FormatUint(number, 10))
		}
		return strings.Join(numbers, ",")
	case []float64:
		numbers := make([]string, 0)
		for _, number := range value.([]float64) {
			numbers = append(numbers, strconv.FormatFloat(number, 'f', -1, 64))
		}
		return strings.Join(numbers, ",")
	case []interface{}:
		values := make([]string, 0)
		for _, str := range value.([]interface{}) {
			values = append(values, ParseString(str))
		}
		return "[" + strings.Join(values, ",") + "]"
	default:
		if value != nil {
			return string(GetJSON(value))
		}
	}
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func MergeStringValues(value interface{}, values ...string) []string {
	array := make([]string, 0)
	switch value.(type) {
	case []string:
		array = value.([]string)
	case string:
		array = append(array, value.(string))
	}
	if len(values) > 0 {
		for _, value := range values {
			if !StringArrayContains(array, value) {
				array = append(array, value)
			}
		}
	}
	return array
}

func StringArrayContains(array []string, value string) bool {
	for _, str := range array {
		if str == value {
			return true
		}
	}
	return false
}

func ContainsSuffix(s string, values []string) bool {
	for _, value := range values {
		if strings.HasSuffix(s, value) {
			return true
		}
	}
	return false
}

func ContainsPrefix(s string, values []string) bool {
	for _, value := range values {
		if strings.HasPrefix(s, value) {
			return true
		}
	}
	return false
}

func ParseErrorSource(str string) (string, string, bool) {
	re := regexp.MustCompile(`<[^<>]+#\d+>`)
	matches := re.FindAllString(str, -1)
	if matches != nil {
		path := strings.Join(matches, "")
		source := strings.ReplaceAll(strings.ReplaceAll(path, "<", ""), ">", "")
		message := strings.Replace(str, path, "", 1)
		return source, message, true
	}
	return "", str, false
}

func ParseURL(rawurl string) (*url.URL, bool) {
	u, err := url.Parse(rawurl)
	if err != nil {
		log.Println(err)
	} else {
		return u, true
	}
	return u, false
}

func ParseMap(value interface{}) iris.Map {
	if value == nil {
		return iris.Map{}
	}
	switch value.(type) {
	case iris.Map:
		return value.(iris.Map)
	case string:
		object := iris.Map{}
		err := jsoniter.UnmarshalFromString(value.(string), &object)
		if err != nil {
			log.Println(err)
		}
		return object
	case []byte:
		object := iris.Map{}
		err := jsoniter.Unmarshal(value.([]byte), &object)
		if err != nil {
			log.Println(err)
		}
		return object
	default:
		object := iris.Map{}
		err := jsoniter.Unmarshal(GetJSON(value), &object)
		if err != nil {
			log.Println(err)
		}
		return object
	}
	return iris.Map{}
}

func GetJSON(value interface{}) []byte {
	if value == nil {
		return make([]byte, 0)
	}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(value)
	if err != nil {
		log.Println(err)
	}
	return bytes
}

func SaveLog(server *serverStruct, ctx iris.Context, body iris.Map) bool {
	if context, client := GetClient("data-analytics", "LogService"); client != nil {
		maintainerId := server.MaintainerId
		clientIP := GetString(server.Config, "client-ip", "127.0.0.1")
		body["id"] = Id()
		body["client_ip"] = clientIP
		body["visibility"] = "internal"
		body["status"] = "active"
		body["maintainer_id"] = maintainerId
		message := ParseString(body["message"])
		prefix := []string{"http", "pg", "sql", "log"}
		if ContainsPrefix(message, prefix) {
			body["topic"] = "application_log"
		} else {
			body["topic"] = "access_log"
		}

		result := iris.Map{}
		err := rpcclient.Call(context, "SaveLog", body, &result)
		if err != nil {
			ctx.Values().Set("LoggerMessage", WrapError(err))
		} else {
			return ParseBool(result["success"])
		}

	}
	config := GetTree(server.Config, "logger")
	if GetBool(config, "use-external-api") {
		body["id"] = Id()
		body["content"] = GetJSON(body["content"])
		body["access_id"] = GetString(config, "access-id")
		body["access_key"] = GetString(config, "access-key")
		request := iris.Map{
			"url": GetString(config, "service-url"),
			"headers": iris.Map{
				"X-Trace-Id": GetTraceId(ctx),
			},
			"body": body,
		}
		if result, ok := PostData(request); ok {
			if _, ok := CheckResponseResult(result); ok {
				return true
			}
		}
		if _, ok := RetryPostData(request, config); ok {
			return true
		}
	}
	return false
}

var (
	ClientConfigMap map[string]ClientConfig
	ClientPoolMap   map[string]*rpcclient.XClientPool
)

type ClientConfig struct {
	PoolSize   int
	FailMode   rpcclient.FailMode
	SelectMode rpcclient.SelectMode
	Discovery  rpcclient.ServiceDiscovery
	Option     rpcclient.Option
}

func GetClient(name string, servicePath string) (context.Context, rpcclient.XClient) {
	ctx := context.Background()
	clientName := name + "/" + servicePath
	clientPool, ok := ClientPoolMap[clientName]
	if !ok {
		clientConfig := new(ClientConfig)
		if config, ok := ClientConfigMap[clientName]; ok {
			clientConfig = &config
		} else if config, ok := ClientConfigMap[name]; ok {
			clientConfig = &config
		} else {
			return ctx, nil
		}
		poolSize := clientConfig.PoolSize
		failMode := clientConfig.FailMode
		selectMode := clientConfig.SelectMode
		discovery := clientConfig.Discovery
		option := clientConfig.Option
		clientPool = rpcclient.NewXClientPool(poolSize, servicePath, failMode, selectMode, discovery, option)
		ClientPoolMap[clientName] = clientPool
	}
	ctx = context.WithValue(ctx, share.ReqMetaDataKey, make(map[string]string))
	return ctx, clientPool.Get()
}

func NewContext() iris.Context {
	return irisContext.NewContext(iris.Default())
}

func Id() string {
	id, err := uuid.NewRandom()
	if err != nil {
		log.Println(err)
	}
	return id.String()

}

func StringifyTime(t time.Time) string {
	layout := "2006-01-02T15:04:05.999999-07:00"
	return t.Format(layout)
}

func WrapError(err error) error {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		return fmt.Errorf("<%s#%v>%w", file, line, err)
	}
	return nil
}

func ParseBool(value interface{}, values ...bool) bool {
	switch value.(type) {
	case bool:
		return value.(bool)
	case string:
		str := value.(string)
		if str != "" {
			truth, err := strconv.ParseBool(str)
			if err != nil {
				log.Println(err)
			} else {
				return truth
			}
		}
	}
	if len(values) > 0 {
		return values[0]
	}
	return false
}

func GetTraceId(ctx iris.Context) string {
	if traceId, ok := ctx.Values().Get("TraceId").(string); ok {
		return strings.TrimPrefix(traceId, "TRACE")
	}
	return ""
}
func PostData(request iris.Map) ([]byte, bool) {
	rawurl := ParseString(request["url"])
	u, err := url.Parse(rawurl)
	if err != nil {
		log.Println(err)
	}
	if u.Scheme == "" {
		return make([]byte, 0), false
	}

	if params, ok := request["url_params"]; ok {
		params := ParseMap(params)
		values := u.Query()
		for key, value := range params {
			values.Set(key, ParseString(value))
		}
		u.RawQuery = values.Encode()
	}

	header := http.Header{}
	header.Set("Content-Type", "application/x-www-form-urlencoded")
	if headers, ok := request["headers"]; ok {
		headers := ParseMap(headers)
		for key, value := range headers {
			header.Set(key, ParseString(value))
		}
	}
	data := make([]byte, 0)
	if body, ok := request["body"]; ok {
		switch header.Get("Content-Type") {
		case "application/x-www-form-urlencoded":
			body := ParseMap(body)
			values := url.Values{}
			for key, value := range body {
				values.Set(key, ParseString(value))
			}
			data = []byte(values.Encode())
		default:
			data = []byte(ParseString(body))
		}
	}
	req, err := http.NewRequest("POST", u.String(), bytes.NewReader(data))
	if err != nil {
		log.Println(err)
		return make([]byte, 0), false
	}
	req.Header = header
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println(err)
		return make([]byte, 0), false
	}
	defer res.Body.Close()
	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println(err)
	} else {
		code := res.StatusCode
		return content, code >= 200 && code < 400
	}
	return make([]byte, 0), false
}

func CheckResponseResult(result []byte) ([]byte, bool) {
	data := []byte(jsoniter.Get(result, "data").ToString())
	success := jsoniter.Get(result, "success")
	switch success.ValueType() {
	case jsoniter.BoolValue:
		return data, success.ToBool() == true
	case jsoniter.StringValue:
		return data, success.ToString() == "true"
	}
	if status := jsoniter.Get(result, "status").ToString(); status != "" {
		return data, strings.ToLower(status) == "success"
	}
	if code := jsoniter.Get(result, "code").ToInt(); code >= 100 && code <= 600 {
		return data, code >= 200 && code < 400
	}
	return data, false
}

func RetryPostData(request iris.Map, config *toml.Tree) ([]byte, bool) {
	mutex := new(sync.Mutex)
	mode := GetString(config, "fail-mode", "failtry")
	retries := GetInt(config, "max-retries")
	if mode == "failfast" || retries == 0 {
		return make([]byte, 0), false
	}
	endpoints := GetStringArray(config, "service-endpoints")
	strategy := GetString(config, "backoff-strategy")
	duration := GetDuration(config, "initial-backoff")
	if duration < time.Millisecond {
		duration = time.Millisecond
	}
	count := 0
	ticks := 0
	ticker := time.NewTicker(duration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ready := true
			ticks += 1
			if strategy == "linear" {
				ready = ticks >= (count+1)*(count+2)/2
			} else if strategy == "exponential" {
				ready = ticks >= (1 << uint(count))
			}
			if ready {
				if count < retries {
					length := len(endpoints)
					if length > 0 && mode == "failover" {
						request["url"] = endpoints[count%length]
					}
					if result, ok := PostData(request); ok {
						if mode == "failover" {
							mutex.Lock()
							config.Set("service-url", request["url"])
							mutex.Unlock()
						}
						return result, true
					}
					count += 1
				} else {
					return make([]byte, 0), false
				}
			}
		}
	}
	return make([]byte, 0), false
}

func GetInt(tree *toml.Tree, key string, values ...int) int {
	value := tree.Get(key)
	if value != nil {
		switch value.(type) {
		case int64:
			return int(value.(int64))
		case uint64:
			return int(value.(uint64))
		case float64:
			return int(value.(float64))
		case string:
			value, err := strconv.ParseInt(value.(string), 10, 64)
			if err != nil {
				log.Println(err)
			} else {
				return int(value)
			}
		}
	}
	if len(values) > 0 {
		return values[0]
	}
	return 0
}

func GetDuration(tree *toml.Tree, key string, values ...time.Duration) time.Duration {
	value := tree.Get(key)
	if value != nil {
		switch value.(type) {
		case string:
			duration, err := time.ParseDuration(value.(string))
			if err != nil {
				log.Println(err)
			} else {
				return duration
			}
		}
	}
	if len(values) > 0 {
		return values[0]
	}
	return 0 * time.Second
}
