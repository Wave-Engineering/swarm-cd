package util

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type StackConfig struct {
	Repo                 string   `mapstructure:"repo" yaml:"repo,omitempty"`
	Branch               string   `mapstructure:"branch" yaml:"branch,omitempty"`
	Tag                  string   `mapstructure:"tag" yaml:"tag,omitempty"`
	ComposeFile          string   `mapstructure:"compose_file" yaml:"compose_file,omitempty"`
	ValuesFile           string   `mapstructure:"values_file" yaml:"values_file,omitempty"`
	SopsFiles            []string `mapstructure:"sops_files" yaml:"sops_files,omitempty"`
	SopsSecretsDiscovery bool     `mapstructure:"sops_secrets_discovery" yaml:"sops_secrets_discovery,omitempty"`
}

type RepoConfig struct {
	Url          string `mapstructure:"url" yaml:"url,omitempty"`
	Username     string `mapstructure:"username" yaml:"username,omitempty"`
	Password     string `mapstructure:"password" yaml:"password,omitempty"`
	PasswordFile string `mapstructure:"password_file" yaml:"password_file,omitempty"`
}

type Config struct {
	ReposPath            string                  `mapstructure:"repos_path"`
	UpdateInterval       int                     `mapstructure:"update_interval"`
	AutoRotate           bool                    `mapstructure:"auto_rotate"`
	StackConfigs         map[string]*StackConfig `mapstructure:"stacks"`
	RepoConfigs          map[string]*RepoConfig  `mapstructure:"repos"`
	SopsSecretsDiscovery bool                    `mapstructure:"sops_secrets_discovery"`
	Address              string                  `mapstructure:"address"`
}

var Configs Config

func LoadConfigs() (err error) {
	err = readConfig()
	if err != nil {
		return fmt.Errorf("could not read configuration file: %w", err)
	}
	if Configs.RepoConfigs == nil {
		err = readRepoConfigs()
		if err != nil {
			return fmt.Errorf("could not read repos file: %w", err)
		}
	}
	if Configs.StackConfigs == nil {
		err = readStackConfigs()
		if err != nil {
			return fmt.Errorf("could not load stacks file: %w", err)
		}
	}
	return
}

func readConfig() (err error) {
	configViper := viper.New()
	configViper.SetConfigName("config")
	configViper.AddConfigPath(".")
	configViper.SetDefault("update_interval", 120)
	configViper.SetDefault("repos_path", "repos")
	configViper.SetDefault("auto_rotate", true)
	configViper.SetDefault("sops_secrets_discovery", false)
	configViper.SetDefault("address", "0.0.0.0:8080")
	err = configViper.ReadInConfig()
	if err != nil && !errors.As(err, &viper.ConfigFileNotFoundError{}) {
		return
	}
	return configViper.Unmarshal(&Configs)
}

func readRepoConfigs() (err error) {
	reposViper := viper.New()
	reposViper.SetConfigName("repos")
	reposViper.AddConfigPath(".")
	err = reposViper.ReadInConfig()
	if err != nil {
		return
	}
	return reposViper.Unmarshal(&Configs.RepoConfigs)
}

func readStackConfigs() (err error) {
	stacksViper := viper.New()
	stacksViper.SetConfigName("stacks")
	stacksViper.AddConfigPath(".")
	err = stacksViper.ReadInConfig()
	if err != nil {
		return
	}
	return stacksViper.Unmarshal(&Configs.StackConfigs)
}

// PersistConfigs writes the current RepoConfigs and StackConfigs to split
// YAML files (repos.yaml and stacks.yaml) using atomic writes. Each file is
// first written to a .tmp suffix, then renamed into place.
func PersistConfigs() error {
	if err := atomicWriteYAML("repos.yaml", Configs.RepoConfigs); err != nil {
		return fmt.Errorf("could not persist repos config: %w", err)
	}
	if err := atomicWriteYAML("stacks.yaml", Configs.StackConfigs); err != nil {
		return fmt.Errorf("could not persist stacks config: %w", err)
	}
	return nil
}

// atomicWriteYAML marshals data to YAML, writes it to path+".tmp", then
// renames the temp file to the final path.
func atomicWriteYAML(path string, data interface{}) error {
	b, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not marshal yaml: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, b, 0644); err != nil {
		return fmt.Errorf("could not write temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("could not rename %s to %s: %w", tmpPath, path, err)
	}
	return nil
}
