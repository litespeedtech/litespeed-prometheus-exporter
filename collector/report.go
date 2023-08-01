package collector

type generalInfoReport struct {
	Version   string
	Uptime    string
	KeyValues map[string]float64
}

type requestRateReport struct {
	VHost     string
	KeyValues map[string]float64
}

type externalAppReport struct {
	AppType   string
	VHost     string
	Handler   string
	KeyValues map[string]float64
}

type litespeedReport struct {
	GeneralInfo generalInfoReport
	ReqRates    []requestRateReport
	ExtApps     []externalAppReport
}

func (lr *litespeedReport) indexOfReqRate(f func(r requestRateReport) bool) int {
	for i, rrReport := range lr.ReqRates {
		if f(rrReport) {
			return i
		}
	}
	return -1
}

func (lr *litespeedReport) indexOfExtApp(f func(r externalAppReport) bool) int {
	for i, eaReport := range lr.ExtApps {
		if f(eaReport) {
			return i
		}
	}
	return -1
}

func (lr *litespeedReport) Add(b litespeedReport) {
	lr.GeneralInfo.Version = b.GeneralInfo.Version
	lr.GeneralInfo.Uptime = b.GeneralInfo.Uptime
	for flag, value := range b.GeneralInfo.KeyValues {
		sumOrAppend(lr.GeneralInfo.KeyValues, flag, value)
	}

	for _, rrReport := range b.ReqRates {
		i := lr.indexOfReqRate(func(r requestRateReport) bool {
			return r.VHost == rrReport.VHost
		})

		if i > -1 {
			for k, v := range rrReport.KeyValues {
				sumOrAppend(lr.ReqRates[i].KeyValues, k, v)
			}
		} else {
			lr.ReqRates = append(lr.ReqRates, rrReport)
		}
	}

	for _, eaReport := range b.ExtApps {
		i := lr.indexOfExtApp(func(r externalAppReport) bool {
			return r.AppType == eaReport.AppType && r.VHost == eaReport.VHost && r.Handler == eaReport.Handler
		})

		if i > -1 {
			for k, v := range eaReport.KeyValues {
				sumOrAppend(lr.ExtApps[i].KeyValues, k, v)
			}
		} else {
			lr.ExtApps = append(lr.ExtApps, eaReport)
		}
	}
}

func sumReports(reports map[string]litespeedReport) *litespeedReport {
	report := &litespeedReport{
		GeneralInfo: generalInfoReport{KeyValues: make(map[string]float64)},
		ReqRates:    []requestRateReport{},
		ExtApps:     []externalAppReport{},
	}

	for _, r := range reports {
		report.Add(r)
	}

	return report
}
