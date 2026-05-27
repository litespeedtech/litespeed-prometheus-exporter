package main

/*
Copyright © 2023-2024 LiteSpeed Technologies <litespeedtech.com>

Licensed under the GPLv3 License (the "License"); you may not use this file
except in compliance with the License.  You may obtain a copy of the License at

    https://www.gnu.org/licenses/gpl-3.0.en.html

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/litespeedtech/litespeed-prometheus-exporter/collector"
	"github.com/spf13/cobra"

	"k8s.io/klog/v2"
)

// Default PID directory. Prefer /run when available (root-owned tmpfs);
// fall back to /tmp/lsws-prometheus-exporter for non-systemd platforms.
const (
	pid_directory_default = "/run/lsws-prometheus-exporter"
	pid_directory_fallback = "/tmp/lsws-prometheus-exporter"
	pid_filename           = "lsws-prometheus-exporter.pid"
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
	// Basic auth flags
	username     = ""
	passwordFile = ""
	// Cgroup command-line flags
	cgroupTry     = 1
	litespeedHome = "/usr/local/lsws"
	rtReport      = "/tmp/lshttpd/.rtreport"
	litespeedPid  = "/tmp/lshttpd/lshttpd.pid"
	pidDirectory  = "" // resolved at runtime to pid_directory_default or fallback
	// Status
	ready = false
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func main() {
	klog.Infof("LiteSpeed Web Server Prometheus Exporter, v%v", version)
	klog.InitFlags(flag.CommandLine)
	defer klog.Flush()

	// math/rand global source is auto-seeded since Go 1.20; no manual Seed needed.

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
		`If you want to require https to access metrics you must specify a tls-cert-file and a tls-key-file which are PEM encoded files`)
	rootCmd.Flags().StringVar(&tlsKeyFile, "tls-key-file", tlsKeyFile,
		`If you want to require https to access metrics you must specify a tls-cert-file and a tls-key-file which are PEM encoded files`)

	rootCmd.Flags().StringVar(&username, "username", username,
		`To enable basic authentication you must specify a username and a secured password file`)
	rootCmd.Flags().StringVar(&passwordFile, "password-file", passwordFile,
		`To enable basic authentication you must specify a username and a secured password file containing the plain-text password`)
	// Backwards-compatible alias for the v0.1.4 spelling.
	rootCmd.Flags().StringVar(&passwordFile, "password_file", passwordFile,
		`Deprecated alias for --password-file (kept for v0.1.4 systemd units)`)

	rootCmd.Flags().IntVar(&cgroupTry, "cgroups", cgroupTry,
		`Whether cgroups v2 user information will be collected.  0 requests disabling, 1 requests enabling if cgroups v2 and LiteSpeed Containers are enabled`)
	rootCmd.Flags().StringVar(&litespeedHome, "litespeed-home", litespeedHome, `Home directory for LiteSpeed.  Defaults to /usr/local/lsws`)
	rootCmd.Flags().StringVar(&rtReport, "rtreport", rtReport, `The fully qualified path for the LiteSpeed real time report file.  Defaults to /tmp/lshttpd/.rtreport`)
	rootCmd.Flags().StringVar(&litespeedPid, "litespeed-pid-file", litespeedPid, `LiteSpeed daemon PID file used for the up/down probe.  Defaults to /tmp/lshttpd/lshttpd.pid`)
	rootCmd.Flags().StringVar(&pidDirectory, "pid-directory", pidDirectory, `Directory for this exporter's PID file. If empty, /run/lsws-prometheus-exporter is used when writable, else /tmp/lsws-prometheus-exporter`)

	if err := rootCmd.Execute(); err != nil {
		klog.Exitf("Exiting due to command-line error: %v", err)
	}
	klog.V(4).Infof("Exiting main()")
}

func run(cmd *cobra.Command, args []string) {
	klog.V(4).Infof("Using build: %v - v%v", gitRepo, version)
	if (tlsCertFile != "" && tlsKeyFile == "") || (tlsCertFile == "" && tlsKeyFile != "") {
		klog.Exitf("You must specify BOTH tls-cert-file AND tls-key-file if you specify either")
	}
	if tlsCertFile != "" {
		if err := validateRegularFile(tlsCertFile); err != nil {
			klog.Exitf("Invalid tls-cert-file: %v", err)
		}
		if err := validateRegularFile(tlsKeyFile); err != nil {
			klog.Exitf("Invalid tls-key-file: %v", err)
		}
		klog.V(4).Info("Access will be via https only")
	}
	if litespeedPid != "" {
		collector.LitespeedPidFile = litespeedPid
	}
	if (username != "" && passwordFile == "") || (username == "" && passwordFile != "") {
		klog.Exitf("You must specify BOTH --username and --password-file if you specify either")
	}
	password := ""
	if passwordFile != "" {
		if err := validateRegularFile(passwordFile); err != nil {
			klog.Exitf("Invalid --password-file: %v", err)
		}
		// Refuse password files that other users can read. The exporter
		// already insists they exist; we additionally insist they are not
		// world/group-readable when running as root.
		if info, err := os.Stat(passwordFile); err == nil {
			if info.Mode().Perm()&0o077 != 0 && os.Geteuid() == 0 {
				klog.Warningf("Password file %v has overly permissive mode %o; recommend chmod 0600", passwordFile, info.Mode().Perm())
			}
		}
		pwd, err := os.ReadFile(passwordFile)
		if err != nil {
			klog.Exitf("Error reading password file %v: %v", passwordFile, err)
		}
		// Trim a single trailing newline (sh/echo leaves one) but otherwise
		// preserve trailing whitespace as part of the password.
		password = string(pwd)
		password = strings.TrimRight(password, "\r\n")
		if password == "" {
			klog.Exitf("Password file %v is empty", passwordFile)
		}
		klog.V(2).Infof("Basic authentication enabled for username %q (password file %v)", username, passwordFile)
	}
	if cgroupTry < 0 || cgroupTry > 2 {
		klog.Exitf("Invalid cgroups value: %v", cgroupTry)
	}
	ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()

	go handleSigterm(cancel)

	createPid()

	collector.Run(ctx, metricsServiceAddr, metricsServicePath, metricsExcludedList, tlsCertFile, tlsKeyFile, username, password, cgroupTry, litespeedHome, rtReport)

	deletePid()
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

// resolvePidDirectory picks an explicit --pid-directory if provided, otherwise
// /run/lsws-prometheus-exporter when it can be created, otherwise the legacy
// /tmp fallback.
func resolvePidDirectory() string {
	if pidDirectory != "" {
		return pidDirectory
	}
	if err := os.MkdirAll(pid_directory_default, 0o755); err == nil {
		return pid_directory_default
	}
	return pid_directory_fallback
}

// pidDirSafe verifies the resolved PID directory is a real directory (not a
// symlink) before we write into it. Returns the absolute, cleaned path.
func pidDirSafe(dir string) (string, error) {
	clean := filepath.Clean(dir)
	if err := os.MkdirAll(clean, 0o755); err != nil {
		return "", err
	}
	info, err := os.Lstat(clean)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", &os.PathError{Op: "lstat", Path: clean, Err: syscall.ELOOP}
	}
	if !info.IsDir() {
		return "", &os.PathError{Op: "lstat", Path: clean, Err: syscall.ENOTDIR}
	}
	return clean, nil
}

func createPid() {
	dir := resolvePidDirectory()
	safeDir, err := pidDirSafe(dir)
	if err != nil {
		klog.Errorf("Cannot use PID directory %v: %v (continuing without pid file)", dir, err)
		return
	}
	pidPath := filepath.Join(safeDir, pid_filename)
	// 0600 — only owner reads. O_EXCL fails if a file (or symlink to a file)
	// already exists, defeating symlink-clobber attacks.
	f, err := os.OpenFile(pidPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_EXCL, 0o600)
	if err != nil {
		// If a stale pid file exists, replace it after a regular-file check.
		if info, lerr := os.Lstat(pidPath); lerr == nil && info.Mode().IsRegular() {
			if rerr := os.Remove(pidPath); rerr == nil {
				f, err = os.OpenFile(pidPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_EXCL, 0o600)
			}
		}
		if err != nil {
			klog.Errorf("Cannot create PID file %v: %v", pidPath, err)
			return
		}
	}
	defer f.Close()
	if _, err := f.WriteString(strconv.Itoa(os.Getpid())); err != nil {
		klog.Errorf("Cannot write PID file %v: %v", pidPath, err)
	}
}

func deletePid() {
	dir := resolvePidDirectory()
	pidPath := filepath.Join(dir, pid_filename)
	if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
		klog.V(4).Infof("os.Remove pid file %v: %v", pidPath, err)
	}
	// Only remove the directory if it's the legacy /tmp fallback we created.
	// /run is owned by systemd and shouldn't be torn down by us.
	if dir == pid_directory_fallback {
		_ = os.Remove(dir)
	}
}

// validateRegularFile ensures the path exists, is a regular file, and is
// readable. Used for TLS cert/key flag validation so we don't accept fifos,
// devices, or directories.
func validateRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return &os.PathError{Op: "stat", Path: path, Err: syscall.EINVAL}
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return f.Close()
}
