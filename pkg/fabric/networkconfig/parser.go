package networkconfig

import (
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// readPEMOrPath reads content from either a PEM string or a file path
func readPEMOrPath(pem, path string) (string, error) {
	if pem != "" {
		return pem, nil
	}
	if path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}
	return "", nil
}

// LoadFromFile loads a network configuration from a YAML file
func LoadFromFile(path string) (*NetworkConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config, err := LoadFromReader(file)
	if err != nil {
		return nil, err
	}

	// Process all certificates and keys
	for orgName, org := range config.Organizations {
		for userName, user := range org.Users {
			// Process user certificate
			if user.Cert.PEM == "" && user.Cert.Path != "" {
				certContent, err := readPEMOrPath(user.Cert.PEM, user.Cert.Path)
				if err != nil {
					return nil, err
				}
				user.Cert.PEM = certContent
			}

			// Process user key
			if user.Key.PEM == "" && user.Key.Path != "" {
				keyContent, err := readPEMOrPath(user.Key.PEM, user.Key.Path)
				if err != nil {
					return nil, err
				}
				user.Key.PEM = keyContent
			}

			org.Users[userName] = user
		}
		config.Organizations[orgName] = org
	}

	// Process peer TLS certificates
	for peerName, peer := range config.Peers {
		if peer.TLSCACerts.PEM == "" && peer.TLSCACerts.Path != "" {
			certContent, err := readPEMOrPath(peer.TLSCACerts.PEM, peer.TLSCACerts.Path)
			if err != nil {
				return nil, err
			}
			peer.TLSCACerts.PEM = certContent
		}
		config.Peers[peerName] = peer
	}

	// Process orderer TLS certificates
	for ordererName, orderer := range config.Orderers {
		if orderer.TLSCACerts.PEM == "" && orderer.TLSCACerts.Path != "" {
			certContent, err := readPEMOrPath(orderer.TLSCACerts.PEM, orderer.TLSCACerts.Path)
			if err != nil {
				return nil, err
			}
			orderer.TLSCACerts.PEM = certContent
		}
		config.Orderers[ordererName] = orderer
	}

	// Process CA TLS certificates
	for caName, ca := range config.CertificateAuthorities {
		if ca.TLSCACerts.PEM == "" && ca.TLSCACerts.Path != "" {
			certContent, err := readPEMOrPath(ca.TLSCACerts.PEM, ca.TLSCACerts.Path)
			if err != nil {
				return nil, err
			}
			ca.TLSCACerts.PEM = certContent
		}
		config.CertificateAuthorities[caName] = ca
	}

	return config, nil
}

// LoadFromReader loads a network configuration from an io.Reader
func LoadFromReader(reader io.Reader) (*NetworkConfig, error) {
	var config NetworkConfig
	decoder := yaml.NewDecoder(reader)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// LoadFromBytes loads a network configuration from a byte slice
func LoadFromBytes(data []byte) (*NetworkConfig, error) {
	var config NetworkConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// SaveToFile saves a network configuration to a YAML file
func (c *NetworkConfig) SaveToFile(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// SaveToWriter saves a network configuration to an io.Writer
func (c *NetworkConfig) SaveToWriter(writer io.Writer) error {
	encoder := yaml.NewEncoder(writer)
	return encoder.Encode(c)
}

// SaveToBytes converts a network configuration to a byte slice
func (c *NetworkConfig) SaveToBytes() ([]byte, error) {
	return yaml.Marshal(c)
}
