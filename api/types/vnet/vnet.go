package vnet

import (
	"net"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
)

const (
	// DefaultIPv4CIDRRange is the range VNet is going to use to assign addresses to resources if the
	// VNet config of the cluster doesn't specify any range.
	DefaultIPv4CIDRRange = "100.64.0.0/10"
)

func NewVnetConfig(spec *vnet.VnetConfigSpec) (*vnet.VnetConfig, error) {
	config := &vnet.VnetConfig{
		Kind:    types.KindVnetConfig,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameVnetConfig,
		},
		Spec: spec,
	}

	if err := ValidateVnetConfig(config); err != nil {
		return nil, trace.Wrap(err)
	}

	return config, nil
}

func ValidateVnetConfig(vnetConfig *vnet.VnetConfig) error {
	if vnetConfig.GetKind() != types.KindVnetConfig {
		return trace.BadParameter("kind must be %q", types.KindVnetConfig)
	}
	if vnetConfig.GetVersion() != types.V1 {
		return trace.BadParameter("version must be %q", types.V1)
	}
	if vnetConfig.GetMetadata().GetName() != types.MetaNameVnetConfig {
		return trace.BadParameter("name must be %q", types.MetaNameVnetConfig)
	}
	if cidrRange := vnetConfig.GetSpec().GetIpv4CidrRange(); cidrRange != "" {
		ip, _, err := net.ParseCIDR(cidrRange)
		if err != nil {
			return trace.Wrap(err, "parsing ipv4_cidr_range")
		}
		if ip4 := ip.To4(); ip4 == nil {
			return trace.BadParameter("ipv4_cidr_range must be valid IPv4")
		}
	}
	for _, zone := range vnetConfig.GetSpec().GetCustomDnsZones() {
		suffix := zone.GetSuffix()
		if len(suffix) == 0 {
			return trace.BadParameter("custom_dns_zone must have a non-empty suffix")
		}
	}
	return nil
}
