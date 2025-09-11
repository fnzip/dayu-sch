package batch

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

const VALID_CONFIG = `
[Interface]
PrivateKey=%s
Address=172.16.0.2/32,2606:4700:110:8598:80bf:9dda:35ac:ba58/128
DNS=1.1.1.1,1.0.0.1,2606:4700:4700::1111,2606:4700:4700::1001
MTU=1280

[Peer]
PublicKey=bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=
AllowedIPs=0.0.0.0/0,::/0
Endpoint=%s
`

// Enum for the different sections in the config file, this is later when parsing a config file to determine the state
type ConfigSection int

const (
	SECTION_INTERFACE ConfigSection = iota
	SECTION_PEER
	SECTION_NONE
)

// Helper type to check if a key is valid for a given section
type ConfigurationKeys []string

func (s ConfigSection) IsElementValid(key string) bool {
	if s == SECTION_INTERFACE {
		return VALID_INTERFACE_KEYS.has(key)
	} else if s == SECTION_PEER {
		return VALID_PEER_KEYS.has(key)
	} else {
		return false
	}
}

func (s ConfigSection) String() string {
	if s == SECTION_INTERFACE {
		return "[Interface]"
	} else if s == SECTION_PEER {
		return "[Peer]"
	} else {
		return "None"
	}
}

// This is probably not a comprehensive list of all valid keys, but its sufficient for most cases
var VALID_INTERFACE_KEYS = ConfigurationKeys{"PrivateKey", "Address", "DNS", "ListenPort", "MTU", "SaveConfig", "PreUp", "PostUp", "PreDown", "PostDown", "Table", "FwMark"}
var VALID_PEER_KEYS = ConfigurationKeys{"PublicKey", "AllowedIPs", "Endpoint", "PersistentKeepalive", "PresharedKey"}

func (keys ConfigurationKeys) has(key string) bool {
	for _, s := range keys {
		if s == key {
			return true
		}
	}
	return false
}

const DEFAULT_MTU = 1420 // MTU is not typically present in WireGuard config files, so a default is provided

// This function reads a configuration and returns the following parsed values:
// - ifaceAddresses: IP addresses with which to configure the local WireGuard interface
// - dnsAddresses: The DNS server to be used by the local WireGuard interface
// - mtu: MTU to be configured for the local WireGuard interface
// - ipcConfig: a string that can be used to configure the WireGuard UAPI via the IPC socket
// If the configuration file is incomplete, e.g. it is missing any fields mandatory for starting the tunnel, an error is returned
// At the moment, only one [Interface] and one [Peer] section is supported, as that is the most common use case
func ParseConfig(config io.Reader) (ifaceAddresses, dnsAddresses []netip.Addr, mtu int, ipcConfig string, err error) {
	var privateKeyPresent, publicKeyPresent, endpointPresent, allowedIpsPresent bool
	var interfaceCount, peerCount int
	var currentSection ConfigSection = SECTION_NONE

	mtu = DEFAULT_MTU

	var ipcConfigBuilder strings.Builder

	lineScanner := bufio.NewScanner(config)

	for lineScanner.Scan() {
		line := strings.TrimSpace(lineScanner.Text())
		if line == "" || line[0] == '#' { // skip empty lines and comments
			continue
		}

		if line == "[Interface]" {
			interfaceCount++
			if interfaceCount > 1 {
				return nil, nil, -1, "", errors.New("Only one [Interface] section is supported at the moment")
			}
			currentSection = SECTION_INTERFACE
			continue
		}

		if line == "[Peer]" {
			peerCount++
			if peerCount > 1 {
				return nil, nil, -1, "", errors.New("Only one [Peer] section is supported at the moment")
			}
			currentSection = SECTION_PEER
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, nil, -1, "", fmt.Errorf("Invalid line in config: %s", lineScanner.Text())
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if !currentSection.IsElementValid(key) {
			return nil, nil, -1, "", fmt.Errorf("Invalid key %s in section %s", key, currentSection.String())
		}

		switch key {
		case "PrivateKey":
			privateKeyBase64 := value

			privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
			if err != nil {
				return nil, nil, -1, "", fmt.Errorf("Error decoding private key: %s", err)
			}
			privateKeyHex := hex.EncodeToString(privateKeyBytes)

			ipcConfigBuilder.WriteString(fmt.Sprintf("private_key=%s\n", privateKeyHex))
			privateKeyPresent = true
		case "Address":
			// split by comma
			addresses := strings.Split(value, ",")
			if len(addresses) == 0 {
				return nil, nil, -1, "", fmt.Errorf("No addresses found in Address field")
			}
			for _, address := range addresses {
				parsedAddress, err := netip.ParsePrefix(address)
				if err != nil {
					return nil, nil, -1, "", fmt.Errorf("Error parsing address: %s", err)
				}
				ifaceAddresses = append(ifaceAddresses, parsedAddress.Addr())
			}
		case "MTU":
			mtu, err = strconv.Atoi(value)
			if err != nil {
				return nil, nil, -1, "", fmt.Errorf("Error parsing MTU: %s", err)
			}
		case "DNS":
			// split by comma
			addresses := strings.Split(value, ",")
			if len(addresses) == 0 {
				return nil, nil, -1, "", fmt.Errorf("No addresses found in DNS field")
			}
			for _, address := range addresses {
				parsedAddress, err := netip.ParseAddr(address)
				if err != nil {
					return nil, nil, -1, "", fmt.Errorf("Error parsing address: %s", err)
				}
				dnsAddresses = append(dnsAddresses, parsedAddress)
			}
		case "PublicKey":
			publicKeyBase64 := value

			publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
			if err != nil {
				return nil, nil, -1, "", fmt.Errorf("Error decoding public key: %s", err)
			}

			publicKeyHex := hex.EncodeToString(publicKeyBytes)

			ipcConfigBuilder.WriteString(fmt.Sprintf("public_key=%s\n", publicKeyHex))
			publicKeyPresent = true
		case "AllowedIPs":
			// split by comma
			allowedIps := strings.Split(value, ",")
			if len(allowedIps) == 0 {
				return nil, nil, -1, "", fmt.Errorf("No allowed IPs found in AllowedIPs field")
			}

			for _, allowedIp := range allowedIps {
				ipcConfigBuilder.WriteString(fmt.Sprintf("allowed_ip=%s\n", allowedIp))
				allowedIpsPresent = true
			}
		case "Endpoint":
			ipcConfigBuilder.WriteString(fmt.Sprintf("endpoint=%s\n", value))
			endpointPresent = true
		}

	}

	// Determine if we have enough information to start the tunnel
	minimalConfigPresent := privateKeyPresent && publicKeyPresent && endpointPresent && allowedIpsPresent && len(dnsAddresses) > 0 && len(ifaceAddresses) > 0
	if !minimalConfigPresent {
		return nil, nil, -1, "", fmt.Errorf("Configuration provided is not sufficient.")
	}

	return ifaceAddresses, dnsAddresses, mtu, ipcConfigBuilder.String(), nil
}

func resolveIPPAndPort(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}

	ip, err := resolveIP(host)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(ip.String(), port), nil
}

func resolveIP(ip string) (*net.IPAddr, error) {
	return net.ResolveIPAddr("ip", ip)
}
