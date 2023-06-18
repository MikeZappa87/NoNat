// Copyright 2017 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This is a sample chained plugin that supports multiple CNI versions. It
// parses prevResult according to the cniVersion
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	plugin "github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/vishvananda/netlink"
)

type PluginConf struct {
	types.NetConf
}

func parseConfig(stdin []byte) (*PluginConf, error) {
	conf := PluginConf{}

	if err := json.Unmarshal(stdin, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse network configuration: %v", err)
	}

	if err := version.ParsePrevResult(&conf.NetConf); err != nil {
		return nil, fmt.Errorf("could not parse prevResult: %v", err)
	}

	return &conf, nil
}

// cmdAdd is called for ADD requests
func cmdAdd(args *skel.CmdArgs) error {
	log.Info().Msg("cni plugin executed started")

	conf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	// run the IPAM plugin and get back the config to apply
	r, err := ipam.ExecAdd(conf.IPAM.Type, args.StdinData)
	if err != nil {
		return err
	}

	// Invoke ipam del if err to avoid ip leak
	defer func() {
		if err != nil {
			ipam.ExecDel(conf.IPAM.Type, args.StdinData)
		}
	}()

	// Convert whatever the IPAM result was into the current Result type
	result, err := current.NewResultFromResult(r)
	if err != nil {
		return err
	}

	if len(result.IPs) == 0 {
		return errors.New("IPAM plugin returned missing IP config")
	}

	root, err := ns.GetCurrentNS()

	if err != nil {
		return err
	}

	defer root.Close()

	veth, err := plugin.RandomVethName()

	if err != nil {
		return err
	}

	//container network namespace
	if err := ns.WithNetNSPath(args.Netns, func(nn ns.NetNS) error {
		veth := &netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{
				Name: args.IfName,
			},
			PeerName:      veth,
			PeerNamespace: netlink.NsFd(int(root.Fd())),
		}

		if err := netlink.LinkAdd(veth); err != nil {
			return err
		}

		for _, v := range result.IPs {
			if err := netlink.AddrAdd(veth, &netlink.Addr{
				IPNet: &v.Address,
			}); err != nil {
				return err
			}
		}

		if err := netlink.LinkSetUp(veth); err != nil {
			return err
		}

		for _, v := range result.Routes {
			if l, err := netlink.LinkByName(args.IfName); err != nil {
				return err
			} else {
				if err := netlink.RouteAdd(&netlink.Route{
					Dst:       &v.Dst,
					LinkIndex: l.Attrs().Index,
				}); err != nil {
					return err
				}
			}
		}

		return nil
	}); err != nil {
		return err
	}

	//root network namespace
	if l, err := netlink.LinkByName(veth); err != nil {
		return err
	} else {
		if err = netlink.LinkSetUp(l); err != nil {
			return err
		}

		for _, v := range result.IPs {
			if err = netlink.RouteAdd(&netlink.Route{

				Dst: &net.IPNet{
					IP:   v.Address.IP,
					Mask: net.CIDRMask(32, 32),
				},
				LinkIndex: l.Attrs().Index,
			}); err != nil {
				return err
			}
		}
	}

	_, err = sysctl.Sysctl(fmt.Sprintf("net/ipv4/conf/%s/proxy_arp", veth), "1")

	if err != nil {
		return err
	}

	log.Info().Msg("cni plugin executed finished")

	return types.PrintResult(result, conf.CNIVersion)
}

// cmdDel is called for DELETE requests
func cmdDel(args *skel.CmdArgs) error {
	conf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	if err := ipam.ExecDel(conf.IPAM.Type, args.StdinData); err != nil {
		return err
	}

	return nil
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	err := skel.PluginMainWithError(cmdAdd, nil, cmdDel, version.All, bv.BuildString("NoNat"))

	if err != nil {
		log.Fatal().Err(err).Msg("This wasnt good. Something bad happened. Please investigate")
	}
}
