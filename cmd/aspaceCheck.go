package cmd

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/nyudlts/go-aspace"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var aspaceEnv string
var aspaceConfigLoc string
var transferInfo TransferInfo

func init() {
	checkCmd.Flags().StringVar(&aspaceConfigLoc, "aspace-config", "", "if not set will default to `/home/'username'/.config/go-aspace.yml")
	checkCmd.Flags().StringVar(&aspaceEnv, "aspace-environment", "prod", "the environment to to lookup in config")
	aspaceCmd.AddCommand(checkCmd)
}

var workOrderLocation string

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check that DOs exist in Archivesspace",
	Run: func(cmd *cobra.Command, args []string) {
		//print bin vers and cmd
		fmt.Printf("ADOC %s ASPACE CHECK\n", version)

		//load project config
		if err := loadProjectConfig(); err != nil {
			panic(err)
		}

		//get aspaceConfig
		if err := getConfig(); err != nil {
			panic(err)
		}

		//get workorder
		if err := findWorkOrder(); err != nil {
			panic(err)
		}

		//get transfer info
		if err := getTransferInfo(); err != nil {
			panic(err)
		}

		//run the check
		if err := aspaceCheck(); err != nil {
			panic(err)
		}
	},
}

func getTransferInfo() error {
	transferInfoLoc := filepath.Join(adocConfig.SIPLoc, "metadata", "transfer-info.txt")
	transferInfoBytes, err := os.ReadFile(transferInfoLoc)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(transferInfoBytes, &transferInfo); err != nil {
		return err
	}

	return nil
}

func getConfig() error {
	if aspaceConfigLoc == "" {
		currentUser, err := user.Current()
		if err != nil {
			return (err)
		}
		aspaceConfigLoc = fmt.Sprintf("/home/%s/.config/go-aspace.yml", currentUser.Username)
	}

	_, err := os.Stat(aspaceConfigLoc)
	if err != nil {
		return err
	}
	return nil
}

func findWorkOrder() error {
	mdDir := filepath.Join(adocConfig.SIPLoc, "metadata")
	var err error
	workOrderFilename, err := getWorkOrderFile(mdDir)
	if err != nil {
		return err
	}
	workOrderLocation = filepath.Join(mdDir, workOrderFilename)
	return nil
}

func aspaceCheck() error {
	client, err := aspace.NewClient(aspaceConfigLoc, aspaceEnv, 20)
	if err != nil {
		panic(err)
	}

	workOrder, _ := os.Open(workOrderLocation)
	defer workOrder.Close()
	wo := aspace.WorkOrder{}
	if err := wo.Load(workOrder); err != nil {
		panic(err)
	}

	var b bytes.Buffer
	out := csv.NewWriter(bufio.NewWriter(&b))
	out.Comma = '\t'
	out.Write([]string{"ao_uri", "title", "do_uri", "do_id", "msg"})
	out.Flush()

	for _, row := range wo.Rows {
		repoId, aoURI, err := aspace.URISplit(row.GetURI())
		if err != nil {
			return err
		}
		fmt.Printf("Checking: %s\n", row.GetURI())

		ao, err := client.GetArchivalObject(repoId, aoURI)
		if err != nil {
			fmt.Printf("ERROR: AO does not exist: %s\n", row.GetURI())
			out.Write([]string{row.GetURI(), "", "", "ERROR: AO does not exist: " + row.GetURI()})
			out.Flush()
			continue
		}

		instances := ao.Instances

		if len(instances) < 1 {
			fmt.Printf("ERROR: AO has no instances: %s\n", row.GetURI())
			out.Write([]string{ao.URI, ao.Title, "ERROR: AO has no instances", ao.ComponentId, "KO"})
			out.Flush()
			continue
		}

		for _, instance := range instances {
			if instance.InstanceType == "digital_object" {
				doURI := instance.DigitalObject["ref"]
				_, doID, err := aspace.URISplit(doURI)
				if err != nil {
					fmt.Printf("ERROR: Not able to split: %s\n", doURI)
					out.Write([]string{row.GetURI(), "", "", "ERROR: Not able to split: " + doURI})
					out.Flush()
					continue
				}

				do, err := client.GetDigitalObject(repoId, doID)
				if err != nil {
					fmt.Printf("ERROR: not able to request: %s\n", doURI)
					out.Write([]string{row.GetURI(), "", "", "ERROR: not able to request: " + doURI})
					out.Flush()
					continue
				}

				if do.DigitalObjectID != row.GetComponentID() {
					fmt.Printf("ERROR: Component IDs do not match: %s, %s, %s\n", row.GetURI(), do.URI, do.DigitalObjectID)
					out.Write([]string{row.GetURI(), do.URI, do.DigitalObjectID, "ERROR: component IDs do not match"})
					out.Flush()
					continue
				} else {
					aoURI := row.GetURI()
					fmt.Printf("%s OK", row.GetURI())
					resourceID := transferInfo.GetResourceID()
					aspaceURI := fmt.Sprintf("https://archivesspace.library.nyu.edu/resources/%s#tree::archival_object_%s", resourceID, getAspaceID(aoURI))
					doIdentifier := getAspaceID(doURI)
					aspaceDOURI := fmt.Sprintf("https://archivesspace.library.nyu.edu/digital_objects/%s#tree::digital_object_%s", doIdentifier, doIdentifier)
					out.Write([]string{aspaceURI, do.Title, aspaceDOURI, do.DigitalObjectID, "OK"})
					out.Flush()
					continue
				}
			}
		}
	}

	checkFilename := filepath.Join("logs", fmt.Sprintf("%s-aspace-check.tsv", adocConfig.CollectionCode))

	if err := os.WriteFile(checkFilename, b.Bytes(), 0775); err != nil {
		panic(err)
	}

	fmt.Println("Checkfile written to:", checkFilename)

	return nil

}

func getAspaceID(aoURI string) string {
	split := strings.Split(aoURI, "/")
	return split[len(split)-1]
}
