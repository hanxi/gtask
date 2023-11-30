package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
)

type Config struct {
	StackBufLen int `json:"stackBufLen"`
	MsgQueueLen int `json:"msgQueueLen"`
}

// 包级别的配置实例，拥有默认值
var C = &Config{
	StackBufLen: 4096,
	MsgQueueLen: 4096,
}

var once sync.Once

// Load 读取和解析配置文件，覆盖默认值
func Load(configPath string) error {
	var err error
	once.Do(func() {
		file, fileErr := os.Open(configPath)
		if fileErr != nil {
			slog.Error("Error opening config file. Using default config", "file", fileErr)
			return
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		if decodeErr := decoder.Decode(C); decodeErr != nil {
			slog.Error("Error decoding config file. Using default config.", "file", decodeErr)
		}
		// TODO: 根据配置修改 log 配置
	})
	return err
}
