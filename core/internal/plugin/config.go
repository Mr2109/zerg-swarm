package plugin

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// PluginConfig 插件配置（plugins.yaml 的单个插件条目）。
type PluginConfig struct {
	Name     string                 `yaml:"name"`
	Type     PluginType             `yaml:"type"`
	Version  string                 `yaml:"version"`
	Enabled  bool                   `yaml:"enabled"`
	Settings map[string]interface{} `yaml:"settings"`
}

// ConfigFile plugins.yaml 根结构。
type ConfigFile struct {
	Plugins []PluginConfig `yaml:"plugins"`
}

// LoadConfig 加载插件配置（简单 YAML 子集解析——不引入外部依赖）。
// 支持格式：
//
//	plugins:
//	  - name: ornith-adapter
//	    type: model-adapter
//	    version: 1.0.0
//	    enabled: true
//	    settings:
//	      temperature: 0.8
func LoadConfig(path string) (*ConfigFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &ConfigFile{}
	scanner := bufio.NewScanner(f)
	var cur *PluginConfig
	inPlugins := false
	inSettings := false
	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "plugins:" {
			inPlugins = true
			continue
		}
		if inPlugins {
			if strings.HasPrefix(line, "- name:") {
				cfg.Plugins = append(cfg.Plugins, PluginConfig{})
				cur = &cfg.Plugins[len(cfg.Plugins)-1]
				cur.Name = strings.TrimSpace(strings.TrimPrefix(line, "- name:"))
				cur.Enabled = true
				cur.Settings = map[string]interface{}{}
				inSettings = false
			} else if cur != nil {
				if strings.HasPrefix(line, "type:") {
					cur.Type = PluginType(strings.TrimSpace(strings.TrimPrefix(line, "type:")))
					inSettings = false
				} else if strings.HasPrefix(line, "version:") {
					cur.Version = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
					inSettings = false
				} else if strings.HasPrefix(line, "enabled:") {
					cur.Enabled = strings.TrimSpace(strings.TrimPrefix(line, "enabled:")) == "true"
					inSettings = false
				} else if strings.HasPrefix(line, "settings:") {
					inSettings = true // settings 子项（下一行开始缩进）
				} else if inSettings {
					// settings 内键值（原始行有缩进）
					kv := strings.SplitN(line, ":", 2)
					if len(kv) == 2 {
						key := strings.TrimSpace(kv[0])
						val := strings.Trim(strings.TrimSpace(kv[1]), "\"'")
						cur.Settings[key] = val
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(cfg.Plugins) == 0 {
		return nil, fmt.Errorf("plugins.yaml 无插件配置")
	}
	return cfg, nil
}
