package iris_logger

import (
	"fmt"
	"github.com/kataras/golog"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/logger"
	"github.com/pelletier/go-toml"
	"os"
	"strings"
	"time"
)

func Logger(serverConfig *toml.Tree, StartTime time.Time, handler iris.Handler, fileOs *os.File, close func() error) {
	config := GetTree(serverConfig, "logger")
	contextKeys := GetStringArray(config, "context-keys")
	headerKeys := GetStringArray(config, "header-keys")
	cfg := logger.Config{
		Status:             true,
		IP:                 true,
		Method:             true,
		Path:               true,
		Query:              true,
		MessageContextKeys: MergeStringValues(contextKeys, "requestId", "LoggerMessage"),
		MessageHeaderKeys:  MergeStringValues(headerKeys, "User-Agent"),
	}

	if config.Has("log-file") {
		logFile := GetString(config, "log-file")
		if strings.Contains(logFile, "％s") {
			logFile = fmt.Sprintf(logFile, StartTime.Format("20060102"))
		}
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0766)
		if err == nil {
			fileOs = file
		}
		close = func() error {
			err := file.Close()
			return err
		}
	}

	keepFailedRequests := GetBool(config, "keep-failed-request", true)
	if config.Has("excluded-methods") {
		cfg.AddSkipper(func(ctx iris.Context) bool {
			if keepFailedRequests && ctx.GetStatusCode() >= 400 {
				return false
			}
			methods := GetStringArray(config, "excluded-methods")
			return StringArrayContains(methods, ctx.Method())
		})
	}
	if config.Has("excluded-routes") {
		cfg.AddSkipper(func(ctx iris.Context) bool {
			if keepFailedRequests && ctx.GetStatusCode() >= 400 {
				return false
			}
			routes := GetStringArray(config, "excluded-routes")
			return StringArrayContains(routes, ctx.Path())
		})
	}
	if config.Has("excluded-parties") {
		cfg.AddSkipper(func(ctx iris.Context) bool {
			if keepFailedRequests && ctx.GetStatusCode() >= 400 {
				return false
			}
			parties := GetStringArray(config, "excluded-parties")
			return ContainsPrefix(ctx.Path(), parties)
		})
	}
	if config.Has("excluded-extensions") {
		cfg.AddSkipper(func(ctx iris.Context) bool {
			if keepFailedRequests && ctx.GetStatusCode() >= 400 {
				return false
			}
			extensions := GetStringArray(config, "excluded-extensions")
			return ContainsSuffix(ctx.Path(), extensions)
		})
	}

	handler = logger.New(cfg)
	close = func() error { return nil }
	golog.ErrorText("[ERROR]", "")
	return
}
