package types

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/machinefi/w3bstream/pkg/depends/base/types"
	"github.com/machinefi/w3bstream/pkg/depends/kit/validator/strfmt"
	"github.com/machinefi/w3bstream/pkg/enums"
)

type UploadConfig struct {
	FilesizeLimitBytes int64 `env:""`
	DiskReserveBytes   int64 `env:""`
}

func (c *UploadConfig) SetDefault() {
	if c.FilesizeLimitBytes == 0 {
		c.FilesizeLimitBytes = 1024 * 1024
	}
	if c.DiskReserveBytes == 0 {
		c.DiskReserveBytes = 20 * 1024 * 1024
	}
}

type FileSystem struct {
	Type enums.FileSystemMode `env:""`
}

func (f *FileSystem) SetDefault() {
	if f.Type > enums.FILE_SYSTEM_MODE__S3 || f.Type <= 0 {
		f.Type = enums.FILE_SYSTEM_MODE__LOCAL
	}
}

type ETHClientConfig struct {
	Endpoints string            `env:""`
	Clients   map[uint32]string `env:"-"`
}

func (c *ETHClientConfig) Init() {
	c.Clients = make(map[uint32]string)
	if !gjson.Valid(c.Endpoints) {
		return
	}
	for k, v := range gjson.Parse(c.Endpoints).Map() {
		chainID, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		url := v.String()
		c.Clients[uint32(chainID)] = url
	}
}

type Chain struct {
	ChainID  uint64          `json:"chainID,omitempty"`
	Name     enums.ChainName `json:"name"`
	Endpoint string          `json:"endpoint"`
}

type ChainConfig struct {
	Configs  string                     `env:""     json:"-"`
	Chains   map[enums.ChainName]*Chain `env:"-"    json:"-"`
	ChainIDs map[uint64]*Chain          `env:"-"    json:"-"`
}

func (c *ChainConfig) Init() {
	cs := []*Chain{}
	if c.Configs != "" {
		if err := json.Unmarshal([]byte(c.Configs), &cs); err != nil {
			panic(err)
		}
	}

	cm := make(map[enums.ChainName]*Chain)
	cidm := make(map[uint64]*Chain)
	for _, c := range cs {
		cm[c.Name] = c
		if c.ChainID != 0 {
			cidm[c.ChainID] = c
		}
	}
	c.Chains = cm
	c.ChainIDs = cidm
}

// aliases from base/types
type (
	SFID                     = types.SFID
	SFIDs                    = types.SFIDs
	EthAddress               = types.EthAddress
	Timestamp                = types.Timestamp
	Initializer              = types.Initializer
	ValidatedInitializer     = types.ValidatedInitializer
	InitializerWith          = types.InitializerWith
	ValidatedInitializerWith = types.ValidatedInitializerWith
)

type EthAddressWhiteList []string

func (v *EthAddressWhiteList) Init() {
	lst := EthAddressWhiteList{}
	for _, addr := range *v {
		if err := strfmt.EthAddressValidator.Validate(addr); err == nil {
			lst = append(lst, strings.ToLower(addr))
		}
	}
	*v = lst
}

func (v *EthAddressWhiteList) Validate(address string) bool {
	if v == nil || len(*v) == 0 {
		return true
	}
	for _, addr := range *v {
		if addr == strings.ToLower(address) {
			return true
		}
	}
	return false
}

type StrategyResult struct {
	ProjectName string     `json:"projectName" db:"f_prj_name"`
	AppletID    types.SFID `json:"appletID"    db:"f_app_id"`
	AppletName  string     `json:"appletName"  db:"f_app_name"`
	InstanceID  types.SFID `json:"instanceID"  db:"f_ins_id"`
	Handler     string     `json:"handler"     db:"f_hdl"`
	EventType   string     `json:"eventType"   db:"f_evt"`
}

type WasmDBConfig struct {
	MaxConnection int
}

func (c *WasmDBConfig) SetDefault() {
	if c.MaxConnection == 0 {
		c.MaxConnection = 2
	}
}

type MetricsCenterConfig struct {
	Endpoint      string `env:""`
	ClickHouseDSN string `env:""`
}

type RobotNotifierConfig struct {
	Vendor string   `env:""` // Vendor robot vendor eg: `Lark` `Wechat Work` `DingTalk`
	Env    string   `env:""` // Env Service env, eg: dev-staging, prod
	URL    string   `env:""` // URL webhook url
	Secret string   `env:""` // Secret message secret
	PINs   []string `env:""` // PINs pin someone

	SignFn func(int64) (string, error) `env:"-"`
}

func (c *RobotNotifierConfig) IsZero() bool { return c == nil || c.URL == "" }

func (c *RobotNotifierConfig) Init() {
	if c.Secret != "" {
		c.SignFn = func(ts int64) (string, error) {
			payload := fmt.Sprintf("%v", ts) + "\n" + c.Secret

			var data []byte
			h := hmac.New(sha256.New, []byte(payload))
			_, err := h.Write(data)
			if err != nil {
				return "", err
			}

			signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
			return signature, nil
		}
	}
}
