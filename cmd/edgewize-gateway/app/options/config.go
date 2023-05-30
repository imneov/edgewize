package options

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"k8s.io/klog"
	"strings"
	"sync"
)

var (
	_viper = viper.New()
	// singleton instance of config package
	_config = defaultConfig()
)

const (
	// DefaultConfigurationName is the default name of configuration
	defaultConfigurationName = "edgewize-servers"
	///Users/neov/src/edgewize/edgewize/edgewize-servers.yaml

	// DefaultConfigurationPath the default location of the configuration file
	defaultConfigurationPath = "/etc/edgewize"
)

func (c *config) watchConfig() <-chan ServerEndpoints {
	c.watchOnce.Do(func() {
		_viper.WatchConfig()
		_viper.OnConfigChange(func(in fsnotify.Event) {
			cfg := New()
			if err := _viper.Unmarshal(cfg); err != nil {
				klog.Warning("config reload error", err)
			} else {
				c.cfgChangeCh <- *cfg
			}
		})
	})
	return c.cfgChangeCh
}

func (c *config) loadFromDisk() (*ServerEndpoints, error) {
	var err error
	c.loadOnce.Do(func() {
		if err = _viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				err = fmt.Errorf("error parsing configuration file %s", err)
			}
		}
		err = _viper.Unmarshal(c.cfg)
	})
	return c.cfg, err
}

func defaultConfig() *config {

	_viper.SetConfigName(defaultConfigurationName)
	_viper.AddConfigPath(defaultConfigurationPath)

	// Load from current working directory, only used for debugging
	_viper.AddConfigPath(".")

	// Load from Environment variables
	_viper.SetEnvPrefix("kubesphere")
	_viper.AutomaticEnv()
	_viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	return &config{
		cfg:         New(),
		cfgChangeCh: make(chan ServerEndpoints),
		watchOnce:   sync.Once{},
		loadOnce:    sync.Once{},
	}
}

type config struct {
	cfg         *ServerEndpoints
	cfgChangeCh chan ServerEndpoints
	watchOnce   sync.Once
	loadOnce    sync.Once
}

// TryLoadFromDisk loads configuration from default location after server startup
// return nil error if configuration file not exists
func TryLoadFromDisk() (*ServerEndpoints, error) {
	return _config.loadFromDisk()
}

// WatchConfigChange return config change channel
func WatchConfigChange() <-chan ServerEndpoints {
	return _config.watchConfig()
}

type ServerEndpoint struct {
	Clusterip string `json:"clusterip,omitempty"`
}

type ServerEndpoints map[string]ServerEndpoint

// newConfig creates a default non-empty Config
func New() *ServerEndpoints {
	return &ServerEndpoints{}
}
