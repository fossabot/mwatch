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

// A simple example exposing fictional RPC latencies with different types of
// random distributions (uniform, normal, and exponential) as Prometheus
// metrics.
package main

//
import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/samuel-sujith/mwatch/pkg/generate"
	"github.com/samuel-sujith/mwatch/pkg/types"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

func main() {

	configuration := types.Cfg{Listenaddress: "a dummy address", DesiredMetric: "a dummy metric"}
	var cert, key string
	var skipServerCertCheck bool

	a := kingpin.New(filepath.Base(os.Args[0]), "The Model metric watcher")

	a.Flag("listenaddress", "Model watcher address to watch.").Default("http://localhost:8080/metrics").StringVar(&configuration.Listenaddress)
	a.Flag("desired_metric", "Desired metric to watch out for").Default("process_open_fds").StringVar(&configuration.DesiredMetric)
	a.Flag("cert", "certificate file for client.").Default("").StringVar(&cert)
	a.Flag("key", "key for certificate file for client.").Default("").StringVar(&key)
	a.Flag("accept-invalid-cert", "Skipping cert check").Default("true").BoolVar(&skipServerCertCheck)

	_, err := a.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error parsing commandline arguments"))
		a.Usage(os.Args[1:])
		os.Exit(2)
	}

	ch := make(chan bool)
	mfChan := make(chan *dto.MetricFamily, 1024)

	//buf := &bytes.Buffer{}
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)

	//fmt.Println("About to run boiiiii")

	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(5 * time.Second)

	go generate.Runmetrics(ch)

	<-ch

	level.Info(logger).Log("msg", "Generated metrics")
	//buffers := pool.New(1e3, 1e6, 3, func(sz int) interface{} { return make([]byte, 0, sz) })
	//b := buffers.Get(1024).([]byte)
	//buf := bytes.NewBuffer(b)
	//fmt.Println("configuration is ", configuration)
mainLoop:
	for {

		select {
		case <-term:
			level.Info(logger).Log("msg", "exiting due to interrupt")
			break mainLoop
		case <-ticker.C:
			/*contenttype, watcherr := watch.Targetwatching(configuration, buf, logger)
			if watcherr == nil {
				b = buf.Bytes()
				level.Info(logger).Log("msg", "nil response from watcher", "ctype", contenttype)

				//fmt.Println("The content type from the watcher server is ", contenttype)
				//fmt.Println("The response from the watcher is ", b)
			}

			if watcherr != nil {
				//fmt.Println("There is error in watching the metrics", watcherr)
				level.Error(logger).Log("msg", "err in target watching", "err", err)
			}*/
			//TODO
			transport, err := makeTransport(cert, key, skipServerCertCheck)
			if err != nil {
				level.Error(logger).Log("msg", "unable to make transport", "error", err)
			}
			if err := prom2json.FetchMetricFamilies(configuration.Listenaddress, mfChan, transport); err != nil {
				level.Error(logger).Log("msg", "Error parsing response", "error", err)
			}
			result := []*prom2json.Family{}
			for mf := range mfChan {
				result = append(result, prom2json.NewFamily(mf))
				if *mf.Name == configuration.DesiredMetric {
					level.Info(logger).Log("msg", "Your desired metric is", "metric", configuration.DesiredMetric, "value", getValue(mf.Metric[0]))
				}

			}

			/*jsonText, err := json.Marshal(result)
			if err != nil {
				level.Error(logger).Log("msg", "Error marshaling json", "error", err)
			}
			if _, err := os.Stdout.Write(jsonText); err != nil {
				level.Error(logger).Log("msg", "Error writing to stdout", "error", err)
			}*/

			mfChan = make(chan *dto.MetricFamily, 1024)

		}
	}
	//fmt.Println("put into the buffer")
	level.Info(logger).Log("msg", "See you next time!")
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
