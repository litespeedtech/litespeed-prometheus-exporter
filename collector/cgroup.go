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

package collector

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	cgroups_namespace = "cgroups"
	cgroupsDir        = "/sys/fs/cgroup"
	rootUID           = "."
)

/* Metrics titles - the key is the scrape name*/
type metricNameMap map[string]metricInfo
type prefixMetricNameMap map[string]metricNameMap

/* Note that this is a subset of all of the possible stats, but I'm only going to collect the ones I like. */
const (
	/* registered with Prometheus with the 'cpu' prefix and scraped from cpu.stat. */
	cpu_prefix  = "cpu"
	usage_usec  = "usage_usec"
	user_usec   = "user_usec"
	system_usec = "system_usec"
	/* registered with Prometheus with the 'io' prefix and scraped from io.stat. */
	io_prefix = "io"
	rbytes    = "rbytes"
	wbytes    = "wbytes"
	rios      = "rios"
	wios      = "wios"
	/* registered with Prometheus with the 'memory' prefix and scraped from memory.* files. */
	memory_prefix  = "memory"
	memory_current = "current"
	swap_current   = "swap_bytes"
	/* registered with Prometheus with the 'pids' prefix and scraped from pids.* files. */
	pids_prefix  = "pids"
	pids_current = "current"
)

var (
	metricNames prefixMetricNameMap
)

type MetricVal struct {
	prefix string
	info   metricInfo
	val    float64
}

type MetricValMap map[string]MetricVal

type CgroupReport struct {
	KeyValues MetricValMap // key is prefix+ScrapeName
}

// LitespeedCollectorCgroup collects LiteSpeed cgroup stats from the given files and exports them as Prometheus metrics
type LitespeedCollectorCgroup struct {
	collector *LitespeedCollector
	enabled   bool
	minUID    int
}

func cgroupName(prefix, scrapeName string) string {
	name := prefix + "_" + scrapeName
	return name
}

func enable(opts *LitespeedCollectorOpts) bool {
	if _, err := os.Stat(cgroupsDir + "/cgroup.controllers"); errors.Is(err, os.ErrNotExist) {
		klog.V(4).Infof("Not cgroups v2")
		return false
	}
	if opts.CgroupTry == 0 {
		klog.V(4).Infof("User requested no cgroups")
		return false
	}
	if opts.CgroupTry == 2 {
		klog.V(4).Infof("User requested cgroups without LS verification")
		return true
	}
	if _, err := os.Stat(opts.LitespeedHome + "/conf/lscntr.txt"); err != nil {
		klog.V(4).Infof("LiteSpeed Containers not enabled; no cgroups (%v)", err)
		return false
	}
	return true
}

func readStatFile(filename string) (float64, error) {
	dat, err := os.ReadFile(filename)
	if err != nil {
		return 0, err
	}
	var val float64
	line := string(dat[:])
	line = strings.TrimRight(line, "\n")
	val, err = strconv.ParseFloat(line, 64)
	return val, err
}

func newCgroupMetric(prefix, name, scrapeName, help string, t prometheus.ValueType) metricInfo {
	fullname := cgroupName(prefix, name)
	return metricInfo{
		Name:       fullname,
		ScrapeName: scrapeName,
		Desc: prometheus.NewDesc(
			prometheus.BuildFQName(
				cgroups_namespace, "", fullname),
			help+" per user",
			[]string{"uid"},
			nil,
		),
		Type: t,
	}
}

func addCgroupMetrics() {
	metricNames = make(prefixMetricNameMap)
	metricNames[cpu_prefix] = make(metricNameMap)
	metricNames[cpu_prefix][usage_usec] = newCgroupMetric(cpu_prefix, "microseconds", usage_usec, "Total CPU usage in microseconds", prometheus.CounterValue)
	metricNames[cpu_prefix][user_usec] = newCgroupMetric(cpu_prefix, "user_microseconds", user_usec, "User-space CPU usage in microseconds", prometheus.CounterValue)
	metricNames[cpu_prefix][system_usec] = newCgroupMetric(cpu_prefix, "system_microseconds", system_usec, "Kernel-space CPU usage in microseconds", prometheus.CounterValue)
	metricNames[io_prefix] = make(metricNameMap)
	metricNames[io_prefix][rbytes] = newCgroupMetric(io_prefix, "read_bytes", rbytes, "Total bytes read", prometheus.CounterValue)
	metricNames[io_prefix][wbytes] = newCgroupMetric(io_prefix, "write_bytes", wbytes, "Total bytes written", prometheus.CounterValue)
	metricNames[io_prefix][rios] = newCgroupMetric(io_prefix, "reads_total", rios, "Total number of reads", prometheus.CounterValue)
	metricNames[io_prefix][wios] = newCgroupMetric(io_prefix, "writes_total", wios, "Total number of writes", prometheus.CounterValue)
	metricNames[memory_prefix] = make(metricNameMap)
	metricNames[memory_prefix][memory_current] = newCgroupMetric(memory_prefix, "bytes", memory_current, "Total amount of memory currently being used", prometheus.GaugeValue)
	metricNames[memory_prefix][swap_current] = newCgroupMetric(memory_prefix, "swap_bytes", swap_current, "Amount of swap memory currently being used", prometheus.GaugeValue)
	metricNames[pids_prefix] = make(metricNameMap)
	metricNames[pids_prefix][pids_current] = newCgroupMetric(pids_prefix, "total", pids_current, "Total number of tasks active", prometheus.GaugeValue)
}

func NewLitespeedCollectorCgroup(collector *LitespeedCollector) *LitespeedCollectorCgroup {
	cg := &LitespeedCollectorCgroup{
		collector: collector,
		enabled:   enable(&collector.options),
	}
	if cg.enabled {
		addCgroupMetrics()
		minUID, err := readStatFile(collector.options.LitespeedHome + "/lsns.conf")
		if err != nil {
			cg.minUID = 1001
		} else {
			cg.minUID = int(minUID)
		}
	}
	klog.V(4).Infof("NewLitespeedCollectorCgroup, enabled: %v, min_uid: %v", cg.enabled, cg.minUID)

	return cg
}

func (c *LitespeedCollectorCgroup) cgroupDescribe(ch chan<- *prometheus.Desc) {
	klog.V(4).Infof("cgroupDescribe")
	for _, metricsMap := range metricNames {
		for _, metric := range metricsMap {
			if c.collector.metricIsTracked(metric.Name) {
				klog.V(4).Infof("cgroupDescribe, tracking %v", metric.Name)
				ch <- metric.Desc
			} else {
				klog.V(4).Infof("cgroupDescribe, metric NOT tracked! %v", metric.Name)
			}
		}
	}
}

func (c *LitespeedCollectorCgroup) scrapeCPU(dir string, report *CgroupReport) error {
	filename := dir + "/cpu.stat"
	file, err := os.Open(filename)
	if err != nil {
		return err
	}

	defer func() {
		file.Close()
		if r := recover(); r != nil {
			err = fmt.Errorf("failed scraping file: %s", r)
			report = nil
		}
	}()

	reader := bufio.NewReader(file)
	var line string
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimRight(line, "\n")
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			continue
		}
		if parts[0] == usage_usec || parts[0] == user_usec || parts[0] == system_usec {
			val, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				klog.V(4).Infof("scrapeCPU, error: %v", err)
				return err
			}
			klog.V(4).Infof("scrapeCPU, adding %v: %v", parts[0], val)
			var metricVal MetricVal
			metricVal.prefix = cpu_prefix
			metricVal.info = metricNames[cpu_prefix][parts[0]]
			metricVal.val = val
			report.KeyValues[cpu_prefix+parts[0]] = metricVal
		}
	}

	if err != io.EOF {
		return err
	}

	return nil
}

func (c *LitespeedCollectorCgroup) scrapeIO(dir string, report *CgroupReport) error {
	filename := dir + "/io.stat"
	file, err := os.Open(filename)
	if err != nil {
		return err
	}

	defer func() {
		file.Close()
		if r := recover(); r != nil {
			err = fmt.Errorf("failed scraping file: %s", r)
			report = nil
		}
	}()

	reader := bufio.NewReader(file)
	var line string
	var newVal float64
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimRight(line, "\n")
		spaceParts := strings.Split(line, " ")
		if len(spaceParts) < 4 {
			continue
		}
		for _, spacePart := range spaceParts {
			equalParts := strings.Split(spacePart, "=")
			if len(equalParts) != 2 {
				continue
			}
			if equalParts[0] == rbytes || equalParts[0] == wbytes || equalParts[0] == rios || equalParts[0] == wios {
				newVal, err = strconv.ParseFloat(equalParts[1], 64)
				if err != nil {
					return err
				}
				klog.V(4).Infof("scrapeIO, adding %v: %v", equalParts[0], newVal)
				if metricVal, ok := report.KeyValues[io_prefix+equalParts[0]]; ok {
					metricVal.prefix = io_prefix
					metricVal.val = newVal + metricVal.val
					metricVal.info = metricNames[io_prefix][equalParts[0]]
					report.KeyValues[io_prefix+equalParts[0]] = metricVal
				} else {
					metricVal.prefix = io_prefix
					metricVal.val = newVal
					metricVal.info = metricNames[io_prefix][equalParts[0]]
					report.KeyValues[io_prefix+equalParts[0]] = metricVal
				}
			}
		}
	}

	if err != io.EOF {
		return err
	}

	return nil
}

func (c *LitespeedCollectorCgroup) scrapeReports(uid string, reports map[string]CgroupReport) error {
	dir := cgroupsDir + "/user.slice"
	if uid != "" {
		dir = dir + "/user-" + uid + ".slice"
	}
	var report CgroupReport
	report.KeyValues = make(MetricValMap)
	klog.V(4).Infof("scrapeReports: %v, uid: %v", dir, uid)
	if err := c.scrapeCPU(dir, &report); err != nil {
		return err
	}
	if err := c.scrapeIO(dir, &report); err != nil {
		return err
	}
	val, err := readStatFile(dir + "/memory.current")
	if err != nil {
		return err
	}
	var metricVal MetricVal
	metricVal.prefix = memory_prefix
	metricVal.val = val
	metricVal.info = metricNames[memory_prefix][memory_current]
	report.KeyValues[memory_prefix+memory_current] = metricVal
	val, err = readStatFile(dir + "/memory.swap.current")
	if err != nil {
		return err
	}
	metricVal.prefix = memory_prefix
	metricVal.val = val
	metricVal.info = metricNames[memory_prefix][swap_current]
	report.KeyValues[memory_prefix+swap_current] = metricVal
	val, err = readStatFile(dir + "/pids.current")
	if err != nil {
		return err
	}
	metricVal.prefix = pids_prefix
	metricVal.val = val
	metricVal.info = metricNames[pids_prefix][pids_current]
	report.KeyValues[pids_prefix+pids_current] = metricVal
	if uid == "" {
		var uids []string
		reports[rootUID] = report
		search := dir + "/user-*.slice"
		if uids, err = filepath.Glob(search); err != nil {
			return err
		}
		klog.V(4).Infof("scrapeReports: did search using %v, found %v files", search, len(uids))
		for _, uid_path := range uids {
			var uidInt int
			uid := uid_path[len(dir)+6 : len(uid_path)-6]
			klog.V(4).Infof("scrapeReports: converted %v to uid: %v", uid_path, uid)
			uidInt, err = strconv.Atoi(uid)
			if err != nil {
				return err
			} else if uidInt >= c.minUID {
				if err = c.scrapeReports(uid, reports); err != nil {
					return err
				}
			} else {
				klog.V(4).Infof("scrapeReports: skip uid %v for < min %v", uidInt, c.minUID)
			}
		}

	} else {
		reports[uid] = report
	}
	return nil
}

func (c *LitespeedCollectorCgroup) cgroupCollect(ch chan<- prometheus.Metric) error {
	klog.V(4).Infof("cgroupCollect")
	reports := make(map[string]CgroupReport)
	err := c.scrapeReports("", reports)
	if err != nil {
		err = fmt.Errorf("failed in cgroup collect: %v", err)
		c.collector.scrapeFailures.Inc()
		klog.V(4).Infof("scrapeReports failed: %v", err)
		return err
	}

	for uid, report := range reports {
		for _, metricVal := range report.KeyValues {
			if metric, ok := metricNames[metricVal.prefix][metricVal.info.ScrapeName]; ok {
				if c.collector.metricIsTracked(metric.Name) {
					klog.V(4).Infof("cgroupMetric: uid: %v, name: %v value: %v", uid, metricVal.info.Name, metricVal.val)
					ch <- prometheus.MustNewConstMetric(metric.Desc, metric.Type, metricVal.val, uid)
				} else {
					klog.V(4).Infof("cgroupMetric SKIP %v", metric.Name)
				}
			} else {
				klog.Errorf("cgroupMetric: could not find metric for prefix %v ScrapeName %v", metricVal.prefix, metricVal.info.ScrapeName)
			}
		}
	}
	return nil
}
