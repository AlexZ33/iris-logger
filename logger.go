package iris_logger

import (
	"crypto/rsa"
	"fmt"
	"github.com/allegro/bigcache"
	"github.com/kataras/golog"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/core/memstore"
	"github.com/kataras/iris/v12/middleware/logger"
	"github.com/pelletier/go-toml"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type serverStruct struct {
	Dir          string
	Env          string
	Name         string
	Version      string
	MasterAddr   string
	ProvidesAuth bool
	MaintainerId string
	StartTime    time.Time
	IrisConfig   iris.Configuration
	Config       *toml.Tree
	Cache        *bigcache.BigCache
	Record       *bigcache.BigCache
	PublicKey    *rsa.PublicKey
	PrivateKey   *rsa.PrivateKey
	Store        *memstore.Store
}

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
	golog.ErrorText("[ERROR]", 1234)
	return
}
func LogHandler(server *serverStruct, app *iris.Application, handler iris.Handler, fileOs *os.File) {
	if fileOs != nil {
		log := app.Logger()
		name := server.Name
		record := server.Record
		config := GetTree(server.Config, "logger")
		host := GetString(server.Config, "server-host", "localhost")
		level := GetString(config, "level", "info")
		logFile := GetString(config, "log-file")
		enableRotation := GetBool(config, "enable-log-rotation", true)
		needLogFileCheck := enableRotation && strings.Contains(logFile, "%s")
		timeFormat := GetString(config, "time-format", "2006-01-02T15:04:05.999999999-07:00")
		pattern := regexp.MustCompile(`^(\d{3})\s+([0-9\.]+\p{Ll}?s)\s+([0-9a-fA-F\.\:]+)\s+(GET|POST)\s+(\S+)\s*(REQUEST [0-9a-f\-]{32,36})?\s+(.+)$`)
		log.SetLevel(level).SetTimeFormat(timeFormat)
		log.Handle(func(l *golog.Log) bool {
			datetime := l.FormatTime()
			level := golog.GetTextForLevel(l.Level, false)
			source, message, ok := ParseErrorSource(l.Message)
			if !ok {
				_, fn, line, _ := runtime.Caller(0)
				source = fmt.Sprintf("%s#%d", fn, line)
				message = strings.TrimSpace(l.Message)
			}
			if strings.Contains(source, "/"+name+"/") {
				substrings := strings.Split(source, "/"+name+"/")
				if len(substrings) > 1 {
					source = name + "/" + substrings[len(substrings)-1]
				}
			}
			format := `{"datetime":"%s","level":"%s","message":"%s","source":"%s"}`
			output := fmt.Sprintf(format, datetime, level, message, source)
			if needLogFileCheck {
				filename := fmt.Sprintf(logFile, l.Time.Format("20060102"))
				if _, err := os.Stat(filename); os.IsNotExist(err) {
					file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0766)
					if err == nil {
						fileOs.Close()
						fileOs = file
					}
				}
			}
			fmt.Fprintln(fileOs, output)
			go func() {
				parts := pattern.FindStringSubmatch(message)
				content := iris.Map{}
				if parts != nil {
					content["status_code"] = parts[1]
					content["response_time"] = parts[2]
					content["request_ip"] = parts[3]
					content["request_method"] = parts[4]
					if url, ok := ParseURL(parts[5]); ok {
						content["request_path"] = url.Path
					}
					if requestId := parts[6]; requestId != "" {
						key := strings.Replace(requestId, "REQUEST ", "request:", 1)
						if value, err := record.Get(key); err == nil {
							for key, value := range ParseMap(value) {
								content[key] = value
							}
							record.Delete(key)
						}
					}
				}
				params := iris.Map{
					"service":     name,
					"server_host": host,
					"recorded_at": datetime,
					"level":       level,
					"message":     message,
					"content":     content,
					"source":      source,
				}
				SaveLog(server, NewContext(), params)
			}()
			return true
		})
	}
	app.UseGlobal(handler)
}
