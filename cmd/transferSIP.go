package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(sourceXferCmd)
}

var sourceXferCmd = &cobra.Command{
	Use: "transfer-sip",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("adoc %s transfer-sip\n", version)
		//load the configuration
		if err := loadProjectConfig(); err != nil {
			panic(err)
		}

		//run
		if err := transferACM(); err != nil {
			panic(err)
		}
	},
}

func transferACM() error {

	//create the logfile
	logFileName := filepath.Join(adocConfig.LogLoc, "rsync", fmt.Sprintf("%s-adoc-acm-transfer-rsync.txt", adocConfig.CollectionCode))
	logFile, err := os.Create(logFileName)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(logFile)
	defer logFile.Close()

	fmt.Printf("Transferring %s to sip directory\n", adocConfig.SourceLoc)
	cmd := exec.Command("rsync", "-rav", adocConfig.SourceLoc, "sip")

	b, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	if _, err := writer.Write(b); err != nil {
		return err
	}

	if err := writer.Flush(); err != nil {
		return err
	}

	fmt.Printf("Transfer complete\n")

	return nil
}
