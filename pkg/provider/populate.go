// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//Package provider -populatemetrics util reads the prometheus endpoints and constructs the map which
//contains the values for their respective metrics.
package provider

//
import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/kubernetes-incubator/custom-metrics-apiserver/pkg/provider"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/samuel-sujith/mwatch/pkg/types"
	k8s_types "k8s.io/apimachinery/pkg/types"

	"github.com/go-kit/kit/log/level"
)

func (p *testingProvider) populatemetrics(intconf types.Interimconfig, cfg types.Cfg) {

	mfChan := make(chan *dto.MetricFamily, 1024)

	level.Info(intconf.Logger).Log("msg", "Scraping to populate")
	transport, err := makeTransport(intconf.Cert, intconf.Key, intconf.SkipServerCertCheck)
	if err != nil {
		level.Error(intconf.Logger).Log("msg", "unable to make transport", "error", err)
	}
	if err := prom2json.FetchMetricFamilies(cfg.Listenaddress, mfChan, transport); err != nil {
		level.Error(intconf.Logger).Log("msg", "Error parsing response", "error", err)
	}
	for mf := range mfChan {
		result := prom2json.NewFamily(mf)
		for i, m := range mf.Metric {
			switch mf.GetType() {
			case dto.MetricType_SUMMARY:
				level.Info(intconf.Logger).Log("msg", "Discarding summary metric", "metricname", *mf.Name)
				/*summaryvalues := prom2json.Summary{}
				summaryvalues = (result.Metrics[i]).(prom2json.Summary)
				fmt.Println("summary", summaryvalues.Labels)
				sumkeyvalue := createKeyValuePairs(summaryvalues.Labels)
				sumkeyvalue = string(sumkeyvalue[0 : len(sumkeyvalue)-1])
				fmt.Println("sumkeyvalue", sumkeyvalue)
				metricLabels, err := labels.ConvertSelectorToLabelsMap(sumkeyvalue)
				if err != nil {
					fmt.Println("Error is ", err.Error())
				}
				fmt.Println("labels", metricLabels)
				for k, v := range summaryvalues.Labels {
					fmt.Println("Key is ", k)
					fmt.Println("value is ", v)
				}
				fmt.Println("number is ", m.GetSummary().GetSampleSum())
				fmt.Println("count is ", m.GetSummary().GetSampleCount())*/
			case dto.MetricType_HISTOGRAM:
				level.Info(intconf.Logger).Log("msg", "Discarding histogram metric", "metricname", *mf.Name)
				/*histvalues := prom2json.Histogram{}
				histvalues = (result.Metrics[i]).(prom2json.Histogram)
				/*fmt.Println(histvalues.Labels)
				fmt.Println("hist", histvalues.Labels)
				histkeyvalue := createKeyValuePairs(histvalues.Labels)
				histkeyvalue = string(histkeyvalue[0 : len(histkeyvalue)-1])
				fmt.Println("histkeyvalue", histkeyvalue)
				metricLabels, err := labels.ConvertSelectorToLabelsMap(histkeyvalue)
				if err != nil {
					fmt.Println("Error is ", err.Error())
				}
				fmt.Println("labels", metricLabels)
				for k, v := range histvalues.Labels {
					fmt.Println("Key is ", k)
					fmt.Println("value is ", v)
				}
				fmt.Println("number is ", m.GetHistogram().GetSampleSum())
				fmt.Println("count is ", m.GetHistogram().GetSampleCount())*/
			default:
				metricvalues := prom2json.Metric{}
				metricvalues = (result.Metrics[i]).(prom2json.Metric)
				level.Info(intconf.Logger).Log("msg", "Populating counter/gauge metric", "metricname", *mf.Name)
				fmt.Println("metric", metricvalues.Labels)
				namespaced := false
				metkeyvalue, namespace, keys := createKeyValuePairs(metricvalues.Labels)
				if len(namespace) > 0 {
					namespaced = true
				}
				//fmt.Println("metkeyvalue", metkeyvalue)
				metkeyvalue = string(metkeyvalue[0 : len(metkeyvalue)-1])
				metricLabels, err := labels.ConvertSelectorToLabelsMap(metkeyvalue)
				if err != nil {
					level.Error(intconf.Logger).Log("msg", "Err in converting labels", "Error", err.Error())
				}
				level.Info(intconf.Logger).Log("msg", "Labels for metric", "metricname", *mf.Name, "Labels", metricLabels, "value", getValue(m))
				info := provider.CustomMetricInfo{
					Metric:     *mf.Name,
					Namespaced: namespaced,
				}

				for k := range keys {
					metricInfo := CustomMetricResource{
						CustomMetricInfo: info,
						NamespacedName:   k,
					}
					p.values[metricInfo] = metricValue{
						labels: metricLabels,
						value:  *resource.NewMilliQuantity(int64(getValue(m)*1000), resource.DecimalSI),
					}

				}

			}
		}

	}
	mfChan = make(chan *dto.MetricFamily, 1024)
}

func makeTransport(
	certificate string, key string,
	skipServerCertCheck bool,
) (*http.Transport, error) {
	// Start with the DefaultTransport for sane defaults.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Conservatively disable HTTP keep-alives as this program will only
	// ever need a single HTTP request.
	transport.DisableKeepAlives = true
	// Timeout early if the server doesn't even return the headers.
	transport.ResponseHeaderTimeout = time.Minute
	tlsConfig := &tls.Config{InsecureSkipVerify: skipServerCertCheck}
	if certificate != "" && key != "" {
		cert, err := tls.LoadX509KeyPair(certificate, key)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
		tlsConfig.BuildNameToCertificate()
	}
	transport.TLSClientConfig = tlsConfig
	return transport, nil
}

func getValue(m *dto.Metric) float64 {
	switch {
	case m.Gauge != nil:
		return m.GetGauge().GetValue()
	case m.Counter != nil:
		return m.GetCounter().GetValue()
	case m.Untyped != nil:
		return m.GetUntyped().GetValue()
	default:
		return 0.
	}
}

func createKeyValuePairs(m map[string]string) (string, string, map[k8s_types.NamespacedName]int) {
	b := new(bytes.Buffer)
	namespaced := false
	namespace := ""

	labels := make(map[k8s_types.NamespacedName]int)
	for key, value := range m {
		fmt.Fprintf(b, "%s=%s,", key, value)
		if key == "namespace" {
			namespace = value
			namespaced = true
		}
	}
	for key := range m {
		if namespaced {
			if key != "namespace" {
				namespacedName := k8s_types.NamespacedName{
					Name:      key,
					Namespace: namespace,
				}
				labels[namespacedName] = 1
			}
		}
	}
	return b.String(), namespace, labels
}
