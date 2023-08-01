package main

/*
Copyright © 2023 LiteSpeed Technologies <litespeedtech.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/klog/v2"

	"github.com/rperper/lsws-prometheus-exporter.git/collector"
)

var (
	// The 2 values below are overwritten during build.
	version = ""
	gitRepo = ""

	// Command-line flags
	defaultSvc          string
	metricsServiceAddr  = ":9936"
	metricsServicePath  = "/metrics"
	metricsExcludedList = ""
	tlsCertFile         = ""
	tlsKeyFile          = ""
	// Status
	ready = false
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func main() {
	klog.Infof("LiteSpeed Web Server Prometheus Exporter, v%v", version)
	klog.InitFlags(flag.CommandLine)
	defer klog.Flush()

	// We use math/rand to choose interval of resync
	rand.Seed(time.Now().UTC().UnixNano())

	// rootCmd represents the base command when called without any subcommands
	rootCmd := &cobra.Command{
		Use:   "lsws-prometheus-exporter",
		Short: "LiteSpeed Web Server Prometheus Exporter",
		Long: `An interface specific to LiteSpeed's Web Server exporting its statistics:

	The command line allows specification of ...`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		Run: run,
	}

	rootCmd.Flags().AddGoFlagSet(flag.CommandLine)

	rootCmd.Flags().StringVar(&metricsServiceAddr, "metrics-service-addr", metricsServiceAddr,
		`The address and port to use to listen for prometheus collection requests within the pod.  Default: :9936 which listens on all addresses with port 9936.`)
	rootCmd.Flags().StringVar(&metricsServicePath, "metrics-service-path", metricsServicePath,
		`The path to service requests on.  Default: /metrics.`)
	rootCmd.Flags().StringVar(&metricsExcludedList, "metrics-excluded-list", metricsExcludedList,
		`Specify a comma separated list of metrics to exclude, using the LiteSpeed scaped name`)
	rootCmd.Flags().StringVar(&tlsCertFile, "tls-cert-file", tlsCertFile,
		`If you want to require http to access metrics you must specify a tls-cert-file and a tls-key-file which are PEM encoded files`)
	rootCmd.Flags().StringVar(&tlsKeyFile, "tls-key-file", tlsKeyFile,
		`If you want to require http to access metrics you must specify a tls-cert-file and a tls-key-file which are PEM encoded files`)

	if err := rootCmd.Execute(); err != nil {
		klog.Exitf("Exiting due to command-line error: %v", err)
	}
	klog.V(4).Infof("About to exit main")
	klog.V(4).Infof("Exiting main()")
}

func run(cmd *cobra.Command, args []string) {
	//klog.Infof("Using build: %v - v%v", gitRepo, version)

	ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()

	go handleSigterm(cancel)

	collector.Run(ctx, metricsServiceAddr, metricsServicePath, metricsExcludedList, tlsCertFile, tlsKeyFile)

	klog.V(4).Infof("main run terminating")
}

func handleSigterm(cancel context.CancelFunc) {
	klog.V(4).Infof("In handleSigterm registering signals")
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-signalChan
	klog.Infof("Received signal: %v, shutting down", sig)
	cancel()
	klog.V(4).Infof("In handleSigterm terminating")
}
