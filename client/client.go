package client

type NetworkConfig struct {
	Name    string
	ChainId uint64
	NodeUrl string
}

var TestnetConfig = NetworkConfig{
	Name:    "testnet",
	ChainId: 1212101,
	NodeUrl: "https://api.testnet.1money.network",
}

// MainnetConfig is for use with mainnet.  There is no faucet for Mainnet, as these are real user assets.
var MainnetConfig = NetworkConfig{
	Name:    "mainnet",
	ChainId: 21210,
	NodeUrl: "https://api.mainnet.1money.network",
}

// NamedNetworks Map from network name to NetworkConfig
var NamedNetworks map[string]NetworkConfig

func init() {
	NamedNetworks = make(map[string]NetworkConfig, 4)
	setNN := func(nc NetworkConfig) {
		NamedNetworks[nc.Name] = nc
	}
	setNN(TestnetConfig)
	setNN(MainnetConfig)
}
