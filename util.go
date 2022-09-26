package iris_logger

import (
	"github.com/pelletier/go-toml"
	"log"
	"strconv"
	"strings"
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
