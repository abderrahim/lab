package config

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"syscall"

	"gopkg.in/yaml.v2"

	"github.com/lighttiger2505/lab/ui"
	"github.com/mitchellh/go-homedir"
)

var ConfigDataTest = `tokens:
  gitlab.ssl.domain1.jp: token1
  gitlab.ssl.domain2.jp: token2
preferreddomains:
- gitlab.ssl.domain1.jp
- gitlab.ssl.domain2.jp
`

var TokensTest = yaml.MapSlice{
	yaml.MapItem{
		Key:   "gitlab.ssl.domain1.jp",
		Value: "token1",
	},
	yaml.MapItem{
		Key:   "gitlab.ssl.domain2.jp",
		Value: "token2",
	},
}

var PreferredDomainTest = []string{
	"gitlab.ssl.domain1.jp",
	"gitlab.ssl.domain2.jp",
}

type ConfigManager struct {
	Path   string
	Config *Config
}

func NewConfigManager() *ConfigManager {
	return NewConfigManagerPath("")
}

func NewConfigManagerPath(path string) *ConfigManager {
	return &ConfigManager{
		Path:   path,
		Config: nil,
	}
}

func (c *ConfigManager) Init() error {
	if c.Path != "" {
		return nil
	}

	path := getConfigPath()
	if !fileExists(path) {
		if err := createConfigFile(path); err != nil {
			return fmt.Errorf("Not exist config: %s", path)
		}
	}
	c.Path = path

	return nil
}

func (c *ConfigManager) Load() (conf *Config, err error) {
	if c.Path == "" {
		return nil, fmt.Errorf("Please initialize config manager")
	}

	file, err := c.open(os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := file.Close(); err != nil {
			err = cerr
		}
	}()

	conf, err = c.read(file)
	if err != nil {
		return nil, err
	}
	c.Config = conf
	return
}

func (c *ConfigManager) open(flag int, perm os.FileMode) (*os.File, error) {
	if !fileExists(c.Path) {
		return nil, fmt.Errorf("Not exist config: Path %s", c.Path)
	}

	file, err := os.OpenFile(c.Path, flag, perm)
	if err != nil {
		return nil, fmt.Errorf("Filed open file. Error: %s", err.Error())
	}
	return file, nil
}

func (c *ConfigManager) read(r io.Reader) (*Config, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("Failed unmarshal yaml. Error: %s", err.Error())
	}

	conf := &Config{}
	if err := yaml.Unmarshal(b, conf); err != nil {
		return nil, fmt.Errorf("Failed unmarshal yaml. \nError: %s \nBuffer: %s", err.Error(), string(b))
	}
	return conf, nil
}

func (c *ConfigManager) Save() error {
	file, err := c.open(os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); err != nil {
			err = cerr
		}
	}()

	if err = c.write(file); err != nil {
		return err
	}

	return nil
}

func (c *ConfigManager) write(writer io.Writer) error {
	out, err := yaml.Marshal(c.Config)
	if err != nil {
		return fmt.Errorf("Failed marshal config. Error: %v", err.Error())
	}

	if _, err = io.WriteString(writer, string(out)); err != nil {
		return fmt.Errorf("Failed write config file. Error: %s", err.Error())
	}
	return nil
}

func (c *ConfigManager) SavePreferredDomain(domain string) error {
	c.Config.addPreferredDomain(domain)
	if err := c.Save(); err != nil {
		return fmt.Errorf("Failed save domain. Error: %s", err.Error())
	}
	return nil
}

func (c *ConfigManager) SaveToken(domain, token string) error {
	c.Config.addToken(domain, token)
	if err := c.Save(); err != nil {
		return fmt.Errorf("Failed save token. Error: %s", err.Error())
	}
	return nil
}

func (c *ConfigManager) GetTokenOnly(domain string) string {
	return c.Config.getToken(domain)
}

func (c *ConfigManager) GetToken(ui ui.Ui, domain string) (string, error) {
	token := c.Config.getToken(domain)
	if token == "" {
		return c.askToken(ui, domain)
	}
	return token, nil
}

func (c *ConfigManager) askToken(ui ui.Ui, domain string) (string, error) {
	token, err := ui.Ask("Please input GitLab private token :")
	if err != nil {
		return "", fmt.Errorf("Failed input private token. %s", err.Error())
	}

	c.Config.addToken(domain, token)
	if err := c.Save(); err != nil {
		return "", fmt.Errorf("Failed update config of private token. %s", err.Error())
	}
	return token, nil
}

func (c *ConfigManager) TopPriorityDomain(domains []string) string {
	for _, domain := range domains {
		if c.Config.hasDomain(domain) {
			return domain
		}
	}
	return ""
}

func (c *ConfigManager) GetTopDomain() string {
	return c.Config.getTopDomain()
}

type Config struct {
	Tokens           yaml.MapSlice
	PreferredDomains []string
}

func getConfigPath() string {
	dir, _ := homedir.Dir()
	filePath := fmt.Sprintf("%s/.labconfig.yml", dir)
	return filePath
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)

	if pathError, ok := err.(*os.PathError); ok {
		if pathError.Err == syscall.ENOTDIR {
			return false
		}
	}

	if os.IsNotExist(err) {
		return false
	}

	return true
}

func createConfigFile(filePath string) error {
	config := Config{}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Failed create config file: %s", err.Error())
	}
	defer file.Close()

	out, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("Failed marshal config: %v", err.Error())
	}

	_, err = file.Write(out)
	if err != nil {
		return fmt.Errorf("Failed write config file: %s", err.Error())
	}

	return nil
}

func (c *Config) getToken(domain string) (token string) {
	for _, mapItem := range c.Tokens {
		if mapItem.Key.(string) == domain {
			token = mapItem.Value.(string)
		}
	}
	return
}

func (c *Config) getTopDomain() string {
	if len(c.PreferredDomains) > 0 {
		return c.PreferredDomains[0]
	}
	return ""
}

func (c *Config) hasDomain(value string) (result bool) {
	result = false
	for _, domain := range c.PreferredDomains {
		if value == domain {
			result = true
		}
	}
	return
}

func (c *Config) addToken(domain string, token string) {
	item := yaml.MapItem{
		Key:   domain,
		Value: token,
	}
	c.Tokens = append(c.Tokens, item)
}

func (c *Config) AddRepository(repository string) {
	c.PreferredDomains = append(c.PreferredDomains, repository)
}

func (c *Config) addPreferredDomain(domain string) {
	c.PreferredDomains = append(c.PreferredDomains, domain)
}
