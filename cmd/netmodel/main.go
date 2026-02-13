package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ndtobs/netmodel/internal/dedup"
	"github.com/ndtobs/netmodel/internal/exporter"
	"github.com/ndtobs/netmodel/internal/gnmi"
	"github.com/ndtobs/netmodel/internal/inventory"
	"github.com/ndtobs/netmodel/internal/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	version = "0.1.0"

	// Global flags
	timeout time.Duration
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "netmodel",
		Short:   "Network data model generator - extract OpenConfig-based YAML from live networks",
		Version: version,
	}

	rootCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "t", 30*time.Second, "timeout for gNMI operations")

	rootCmd.AddCommand(exportCmd())
	rootCmd.AddCommand(featuresCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func exportCmd() *cobra.Command {
	var (
		username      string
		password      string
		insecure      bool
		features      []string
		outPath       string
		inventoryFile string
		noSplit       bool
		structure     string
		deduplicate   bool
	)

	cmd := &cobra.Command{
		Use:   "export <target>",
		Short: "Export network configuration to YAML data model",
		Long: `Export configuration from network devices via gNMI to a YAML data model.

Target can be:
  - A single device: spine1:6030
  - A group from inventory: @spine (requires inventory file)
  - All devices: @all (requires inventory file)

Output structures:
  - flat: Per-device directories with feature files (default)
  - ansible: group_vars/host_vars layout for Ansible integration

Deduplication (--dedup):
  When exporting multiple devices with --structure ansible, extracts common
  configuration to group_vars. Config identical across ALL devices goes to
  group_vars/all.yaml. Config identical within inventory groups goes to
  group_vars/<group>.yaml. Device-specific config stays in host_vars.

Examples:
  netmodel export 10.0.0.1:6030
  netmodel export spine1:6030 --features interfaces,bgp
  netmodel export spine1:6030 -o spine1.yaml
  netmodel export @spine -i inventory.yaml -o ./network-model/
  netmodel export @all -i inventory.yaml -o ./network-model/ --structure ansible
  netmodel export @all -i inventory.yaml -o ./network-model/ --structure ansible --dedup`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExport(args[0], features, username, password, insecure, outPath, inventoryFile, !noSplit, structure, deduplicate)
		},
	}

	cmd.Flags().StringVarP(&username, "username", "u", "", "username for gNMI authentication")
	cmd.Flags().StringVarP(&password, "password", "P", "", "password for gNMI authentication")
	cmd.Flags().BoolVarP(&insecure, "insecure", "k", false, "skip TLS verification")
	cmd.Flags().StringSliceVarP(&features, "features", "f", nil, "features to export (interfaces,bgp,system). Default: all")
	cmd.Flags().StringVarP(&outPath, "output", "o", "", "output path (file or directory)")
	cmd.Flags().StringVarP(&inventoryFile, "inventory", "i", "", "inventory file for group targets")
	cmd.Flags().BoolVar(&noSplit, "no-split", false, "single file per device (default: split into per-feature files)")
	cmd.Flags().StringVarP(&structure, "structure", "s", "flat", "output structure: flat, ansible")
	cmd.Flags().BoolVar(&deduplicate, "dedup", false, "extract common config to group_vars (requires --structure ansible)")

	return cmd
}

func featuresCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "features",
		Short: "List available export features",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Available features:")
			for _, f := range exporter.List() {
				fmt.Printf("  - %s\n", f)
			}
		},
	}
}

func runExport(target string, features []string, username, password string, insecure bool, outPath, inventoryFile string, split bool, structure string, deduplicate bool) error {
	// Validate dedup flag
	if deduplicate && structure != "ansible" {
		return fmt.Errorf("--dedup requires --structure ansible")
	}

	// Load inventory if needed
	var inv *inventory.Inventory
	var targets []string
	var groups map[string][]string // For dedup: maps group name to hostnames
	var err error

	if strings.HasPrefix(target, "@") {
		groupName := strings.TrimPrefix(target, "@")

		if inventoryFile != "" {
			inv, err = inventory.Load(inventoryFile)
			if err != nil {
				return fmt.Errorf("load inventory: %w", err)
			}
		} else {
			inv, _, err = inventory.AutoDiscover()
			if err != nil {
				return fmt.Errorf("auto-discover inventory: %w", err)
			}
			if inv == nil {
				return fmt.Errorf("target %s requires inventory - create inventory.yaml or pass -i", target)
			}
		}

		hosts, ok := inv.GetGroup(groupName)
		if !ok {
			return fmt.Errorf("group %q not found in inventory (available: %s)", groupName, strings.Join(inv.ListGroups(), ", "))
		}
		if len(hosts) == 0 {
			return fmt.Errorf("group %q is empty", groupName)
		}
		targets = hosts

		// Build groups map for dedup (all groups from inventory)
		if deduplicate {
			groups = make(map[string][]string)
			for _, g := range inv.ListGroups() {
				if g != "all" { // Skip "all" - that's handled separately
					if members, ok := inv.GetGroup(g); ok {
						groups[g] = members
					}
				}
			}
		}
	} else {
		targets = []string{target}
	}

	// Apply inventory defaults if CLI flags not provided
	if inv != nil {
		if username == "" && inv.Defaults.Username != "" {
			username = inv.Defaults.Username
		}
		if password == "" && inv.Defaults.Password != "" {
			password = inv.Defaults.Password
		}
		if !insecure && inv.Defaults.Insecure {
			insecure = true
		}
	}

	// Default to all features
	if len(features) == 0 {
		features = exporter.List()
	}

	// Validate features
	validFeatures := make(map[string]bool)
	for _, f := range exporter.List() {
		validFeatures[f] = true
	}
	for _, f := range features {
		if !validFeatures[f] {
			return fmt.Errorf("unknown feature: %s (available: %s)", f, strings.Join(exporter.List(), ", "))
		}
	}

	// Export each target
	models := make(map[string]*model.DeviceModel)
	for _, t := range targets {
		fmt.Fprintf(os.Stderr, "Exporting from %s...\n", t)

		dm, err := exportDevice(t, features, username, password, insecure)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			continue
		}

		// Add metadata
		dm.Metadata.ExportedAt = time.Now().UTC()
		dm.Metadata.NetmodelVersion = version

		models[t] = dm
		fmt.Fprintf(os.Stderr, "  Done\n")
	}

	if len(models) == 0 {
		return fmt.Errorf("no devices exported successfully")
	}

	// Output
	return writeOutput(models, outPath, split, structure, deduplicate, groups)
}

func exportDevice(target string, features []string, username, password string, insecure bool) (*model.DeviceModel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := gnmi.NewClient(gnmi.Config{
		Address:  target,
		Username: username,
		Password: password,
		Insecure: insecure,
		Timeout:  timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	dm, err := exporter.ExportAll(ctx, client, features)
	if err != nil {
		return nil, err
	}

	return dm, nil
}

func writeOutput(models map[string]*model.DeviceModel, outPath string, split bool, structure string, deduplicate bool, groups map[string][]string) error {
	// If no output path, write to stdout
	if outPath == "" {
		return writeToStdout(models)
	}

	// Handle different output structures
	switch structure {
	case "ansible":
		if deduplicate && len(models) > 1 {
			return writeAnsibleStructureDedup(models, outPath, split, groups)
		}
		return writeAnsibleStructure(models, outPath, split)
	default: // "flat"
		// Check if output is a directory (ends with / or multiple targets)
		isDir := strings.HasSuffix(outPath, "/") || len(models) > 1 || split
		if isDir {
			return writeToDirectory(models, outPath, split)
		}
		return writeToFile(models, outPath)
	}
}

func writeToStdout(models map[string]*model.DeviceModel) error {
	// If single model, output directly
	if len(models) == 1 {
		for _, dm := range models {
			return outputYAML(os.Stdout, dm)
		}
	}

	// Multiple models - output as map
	out := make(map[string]*model.DeviceModel)
	for target, dm := range models {
		// Use hostname if available, otherwise target
		key := dm.Metadata.Hostname
		if key == "" {
			key = sanitizeFilename(target)
		}
		out[key] = dm
	}

	return outputYAML(os.Stdout, out)
}

func writeToFile(models map[string]*model.DeviceModel, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	// Single model
	if len(models) == 1 {
		for _, dm := range models {
			if err := outputYAML(f, dm); err != nil {
				return err
			}
		}
	} else {
		// Multiple models
		out := make(map[string]*model.DeviceModel)
		for target, dm := range models {
			key := dm.Metadata.Hostname
			if key == "" {
				key = sanitizeFilename(target)
			}
			out[key] = dm
		}
		if err := outputYAML(f, out); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "Wrote %s\n", path)
	return nil
}

func writeToDirectory(models map[string]*model.DeviceModel, dir string, split bool) error {
	// Create directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	for target, dm := range models {
		// Use hostname if available
		name := dm.Metadata.Hostname
		if name == "" {
			name = sanitizeFilename(target)
		}

		if split {
			// Create per-device directory with per-feature files
			deviceDir := filepath.Join(dir, name)
			if err := os.MkdirAll(deviceDir, 0755); err != nil {
				return fmt.Errorf("create device directory: %w", err)
			}

			// Write metadata
			if err := writeFeatureFile(deviceDir, "metadata", dm.Metadata); err != nil {
				return err
			}

			// Write interfaces
			if dm.Interfaces != nil {
				if err := writeFeatureFile(deviceDir, "interfaces", map[string]interface{}{"interfaces": dm.Interfaces}); err != nil {
					return err
				}
			}

			// Write BGP
			if dm.BGP != nil {
				if err := writeFeatureFile(deviceDir, "bgp", map[string]interface{}{"bgp": dm.BGP}); err != nil {
					return err
				}
			}

			// Write system
			if dm.System != nil {
				if err := writeFeatureFile(deviceDir, "system", map[string]interface{}{"system": dm.System}); err != nil {
					return err
				}
			}

			// Write routing policy
			if dm.RoutingPolicy != nil {
				if err := writeFeatureFile(deviceDir, "routing_policy", map[string]interface{}{"routing_policy": dm.RoutingPolicy}); err != nil {
					return err
				}
			}

			fmt.Fprintf(os.Stderr, "Wrote %s/\n", deviceDir)
		} else {
			// Single file per device
			path := filepath.Join(dir, name+".yaml")
			f, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}

			if err := outputYAML(f, dm); err != nil {
				f.Close()
				return err
			}
			f.Close()

			fmt.Fprintf(os.Stderr, "Wrote %s\n", path)
		}
	}

	return nil
}

func writeAnsibleStructure(models map[string]*model.DeviceModel, dir string, split bool) error {
	// Create directory structure
	hostVarsDir := filepath.Join(dir, "host_vars")
	groupVarsDir := filepath.Join(dir, "group_vars")

	if err := os.MkdirAll(hostVarsDir, 0755); err != nil {
		return fmt.Errorf("create host_vars directory: %w", err)
	}
	if err := os.MkdirAll(groupVarsDir, 0755); err != nil {
		return fmt.Errorf("create group_vars directory: %w", err)
	}

	// Write each device to host_vars
	for target, dm := range models {
		name := dm.Metadata.Hostname
		if name == "" {
			name = sanitizeFilename(target)
		}

		if split {
			// Create per-device directory with per-feature files
			deviceDir := filepath.Join(hostVarsDir, name)
			if err := os.MkdirAll(deviceDir, 0755); err != nil {
				return fmt.Errorf("create device directory: %w", err)
			}

			// Write metadata
			if err := writeFeatureFile(deviceDir, "metadata", dm.Metadata); err != nil {
				return err
			}

			// Write interfaces
			if dm.Interfaces != nil {
				if err := writeFeatureFile(deviceDir, "interfaces", map[string]interface{}{"interfaces": dm.Interfaces}); err != nil {
					return err
				}
			}

			// Write BGP
			if dm.BGP != nil {
				if err := writeFeatureFile(deviceDir, "bgp", map[string]interface{}{"bgp": dm.BGP}); err != nil {
					return err
				}
			}

			// Write system
			if dm.System != nil {
				if err := writeFeatureFile(deviceDir, "system", map[string]interface{}{"system": dm.System}); err != nil {
					return err
				}
			}

			// Write routing policy
			if dm.RoutingPolicy != nil {
				if err := writeFeatureFile(deviceDir, "routing_policy", map[string]interface{}{"routing_policy": dm.RoutingPolicy}); err != nil {
					return err
				}
			}

			fmt.Fprintf(os.Stderr, "Wrote %s/\n", deviceDir)
		} else {
			// Single file per device
			path := filepath.Join(hostVarsDir, name+".yaml")
			f, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}

			if err := outputYAML(f, dm); err != nil {
				f.Close()
				return err
			}
			f.Close()

			fmt.Fprintf(os.Stderr, "Wrote %s\n", path)
		}
	}

	// Create placeholder for group_vars/all.yaml (for future deduplication)
	allVarsPath := filepath.Join(groupVarsDir, "all.yaml")
	if _, err := os.Stat(allVarsPath); os.IsNotExist(err) {
		placeholder := map[string]interface{}{
			"_netmodel": map[string]interface{}{
				"note":    "Common variables will be extracted here in future versions",
				"version": version,
			},
		}
		f, err := os.Create(allVarsPath)
		if err != nil {
			return fmt.Errorf("create all.yaml: %w", err)
		}
		outputYAML(f, placeholder)
		f.Close()
		fmt.Fprintf(os.Stderr, "Wrote %s (placeholder)\n", allVarsPath)
	}

	return nil
}

func writeAnsibleStructureDedup(models map[string]*model.DeviceModel, dir string, split bool, groups map[string][]string) error {
	// Create directory structure
	hostVarsDir := filepath.Join(dir, "host_vars")
	groupVarsDir := filepath.Join(dir, "group_vars")

	if err := os.MkdirAll(hostVarsDir, 0755); err != nil {
		return fmt.Errorf("create host_vars directory: %w", err)
	}
	if err := os.MkdirAll(groupVarsDir, 0755); err != nil {
		return fmt.Errorf("create group_vars directory: %w", err)
	}

	// Build hostname-keyed models (dedup uses hostnames)
	hostnameModels := make(map[string]*model.DeviceModel)
	for target, dm := range models {
		name := dm.Metadata.Hostname
		if name == "" {
			name = sanitizeFilename(target)
		}
		hostnameModels[name] = dm
	}

	// Convert groups to use hostnames
	hostnameGroups := make(map[string][]string)
	for groupName, targets := range groups {
		var hostnames []string
		for _, t := range targets {
			// Find the hostname for this target
			if dm, ok := models[t]; ok && dm.Metadata.Hostname != "" {
				hostnames = append(hostnames, dm.Metadata.Hostname)
			} else {
				hostnames = append(hostnames, sanitizeFilename(t))
			}
		}
		hostnameGroups[groupName] = hostnames
	}

	// Run deduplication
	result := dedup.Deduplicate(hostnameModels, hostnameGroups)

	// Write group_vars/all.yaml (common config)
	if result.Common != nil && !isEmptyModel(result.Common) {
		allVarsPath := filepath.Join(groupVarsDir, "all.yaml")
		if err := writeModelFile(allVarsPath, result.Common, split); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Wrote %s (common config)\n", allVarsPath)
	}

	// Write group_vars/<group>.yaml for each group with common config
	for groupName, groupModel := range result.GroupCommon {
		if groupModel != nil && !isEmptyModel(groupModel) {
			groupPath := filepath.Join(groupVarsDir, groupName+".yaml")
			if err := writeModelFile(groupPath, groupModel, false); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Wrote %s (group common)\n", groupPath)
		}
	}

	// Write host_vars/<host>/ for each device (host-specific config only)
	for hostname, hostModel := range result.HostSpecific {
		if hostModel == nil || isEmptyModel(hostModel) {
			fmt.Fprintf(os.Stderr, "Skipped %s (all config in group_vars)\n", hostname)
			continue
		}

		if split {
			deviceDir := filepath.Join(hostVarsDir, hostname)
			if err := os.MkdirAll(deviceDir, 0755); err != nil {
				return fmt.Errorf("create device directory: %w", err)
			}

			if err := writeModelSplit(deviceDir, hostModel); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Wrote %s/ (host-specific)\n", deviceDir)
		} else {
			hostPath := filepath.Join(hostVarsDir, hostname+".yaml")
			if err := writeModelFile(hostPath, hostModel, false); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Wrote %s (host-specific)\n", hostPath)
		}
	}

	return nil
}

func writeModelFile(path string, dm *model.DeviceModel, split bool) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	// Build output map with only non-nil fields
	out := make(map[string]interface{})

	if dm.System != nil {
		out["system"] = dm.System
	}
	if dm.BGP != nil {
		out["bgp"] = dm.BGP
	}
	if dm.Interfaces != nil {
		out["interfaces"] = dm.Interfaces
	}
	if dm.OSPF != nil {
		out["ospf"] = dm.OSPF
	}
	if dm.EVPN != nil {
		out["evpn"] = dm.EVPN
	}
	if dm.RoutingPolicy != nil {
		out["routing_policy"] = dm.RoutingPolicy
	}

	return outputYAML(f, out)
}

func writeModelSplit(dir string, dm *model.DeviceModel) error {
	// Write metadata if present
	if dm.Metadata.Hostname != "" || dm.Metadata.Model != "" {
		if err := writeFeatureFile(dir, "metadata", dm.Metadata); err != nil {
			return err
		}
	}

	if dm.Interfaces != nil {
		if err := writeFeatureFile(dir, "interfaces", map[string]interface{}{"interfaces": dm.Interfaces}); err != nil {
			return err
		}
	}
	if dm.BGP != nil {
		if err := writeFeatureFile(dir, "bgp", map[string]interface{}{"bgp": dm.BGP}); err != nil {
			return err
		}
	}
	if dm.System != nil {
		if err := writeFeatureFile(dir, "system", map[string]interface{}{"system": dm.System}); err != nil {
			return err
		}
	}
	if dm.OSPF != nil {
		if err := writeFeatureFile(dir, "ospf", map[string]interface{}{"ospf": dm.OSPF}); err != nil {
			return err
		}
	}
	if dm.EVPN != nil {
		if err := writeFeatureFile(dir, "evpn", map[string]interface{}{"evpn": dm.EVPN}); err != nil {
			return err
		}
	}
	if dm.RoutingPolicy != nil {
		if err := writeFeatureFile(dir, "routing_policy", map[string]interface{}{"routing_policy": dm.RoutingPolicy}); err != nil {
			return err
		}
	}
	return nil
}

func isEmptyModel(dm *model.DeviceModel) bool {
	if dm == nil {
		return true
	}
	return dm.Interfaces == nil && dm.BGP == nil && dm.OSPF == nil &&
		dm.EVPN == nil && dm.System == nil && dm.RoutingPolicy == nil
}

func writeFeatureFile(dir, feature string, data interface{}) error {
	path := filepath.Join(dir, feature+".yaml")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	return outputYAML(f, data)
}

func outputYAML(w *os.File, v interface{}) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(v)
}

func sanitizeFilename(s string) string {
	// Remove port
	if idx := strings.LastIndex(s, ":"); idx != -1 {
		s = s[:idx]
	}
	// Replace dots and other chars
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, "/", "-")
	return s
}
