// Package exporter provides functions to extract config from devices via gNMI
package exporter

import (
	"context"

	"github.com/ndtobs/netmodel/internal/gnmi"
	"github.com/ndtobs/netmodel/internal/model"
)

// Exporter extracts configuration from a device
type Exporter interface {
	Name() string
	Export(ctx context.Context, client *gnmi.Client) error
	Apply(m *model.DeviceModel)
}

// List returns all available exporter names
func List() []string {
	return []string{"interfaces", "bgp", "ospf", "system", "routing_policy"}
}

// Get returns an exporter by name
func Get(name string) Exporter {
	switch name {
	case "interfaces":
		return &InterfacesExporter{}
	case "bgp":
		return &BGPExporter{}
	case "ospf":
		return &OSPFExporter{}
	case "system":
		return &SystemExporter{}
	case "routing_policy":
		return &RoutingPolicyExporter{}
	default:
		return nil
	}
}

// ExportAll runs all specified exporters and builds a DeviceModel
func ExportAll(ctx context.Context, client *gnmi.Client, exporters []string) (*model.DeviceModel, error) {
	dm := &model.DeviceModel{}

	for _, name := range exporters {
		exp := Get(name)
		if exp == nil {
			continue
		}

		if err := exp.Export(ctx, client); err != nil {
			// Log but continue with other exporters
			continue
		}

		exp.Apply(dm)
	}

	return dm, nil
}
