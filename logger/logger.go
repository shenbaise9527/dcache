package logger

import (
	"io"
	"os"
	"path/filepath"
	"time"

	rotatelogs "github.com/lestrrat/go-file-rotatelogs"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
)

var (
	logger    *logrus.Logger
	logWriter *rotatelogs.RotateLogs
)

// NewLogger 创建日志对象.
func NewLogger(logPath string, logLevel uint32) error {
	// 创建目录.
	err := os.MkdirAll(logPath, os.ModePerm)
	if err != nil {
		return err
	}

	logger = logrus.New()
	logName := filepath.Join(logPath, "dcache")
	//linkName := logName + ".log"

	// 显示行号等信息.
	logger.SetReportCaller(true)

	// 禁止logrus的输出.
	src, err := os.OpenFile(os.DevNull, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}

	logger.SetOutput(src)

	// 设置日志级别.
	logger.SetLevel(logrus.Level(logLevel))

	// 设置分割规则.
	logWriter, err = rotatelogs.New(
		// 分割后的文件名.
		logName+"_%Y-%m-%d.log",

		// 设置文件软连接,方便找到当前日志文件.
		//rotatelogs.WithLinkName(linkName),

		// 设置文件清理前的最长保存时间,参数=-1表示不清除.
		rotatelogs.WithMaxAge(7*24*time.Hour),

		// 设置文件清理前最多保存的个数,不能与WithMaxAge同时使用.
		//rotatelogs.WithRotationCount(100),

		// 设置日志分割时间,这里设置24小时分割一次.
		rotatelogs.WithRotationTime(24*time.Hour),
	)

	writerMap := lfshook.WriterMap{
		logrus.PanicLevel: logWriter,
		logrus.FatalLevel: logWriter,
		logrus.ErrorLevel: logWriter,
		logrus.WarnLevel:  logWriter,
		logrus.InfoLevel:  logWriter,
		logrus.DebugLevel: logWriter,
		logrus.TraceLevel: logWriter,
	}

	lfHook := lfshook.NewHook(writerMap, &logrus.TextFormatter{
		// 格式化输出时间.
		TimestampFormat: "2006-01-02 15:04:05",
	})

	logger.AddHook(lfHook)

	return err
}

// GetLoggerWriter 获取日志writer.
func GetLoggerWriter() io.Writer {
	return logWriter
}

// Tracef 打印日志.
func Tracef(format string, args ...interface{}) {
	logger.Tracef(format, args...)
}

// Debugf 打印日志.
func Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

// Infof 打印日志.
func Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

// Warnf 打印日志.
func Warnf(format string, args ...interface{}) {
	logger.Warnf(format, args...)
}

// Errorf 打印日志.
func Errorf(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}

// Fatalf 打印日志.
func Fatalf(format string, args ...interface{}) {
	logger.Fatalf(format, args...)
}

// Panicf 打印日志.
func Panicf(format string, args ...interface{}) {
	logger.Panicf(format, args...)
}
