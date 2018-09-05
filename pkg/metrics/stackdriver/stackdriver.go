// Copyright © 2018 Aviv Laufer <aviv.laufer@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package stackdriver

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/kuberbs/pkg/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	monitoring "google.golang.org/api/monitoring/v3"
)

// checkMetrics - connect to stackdriver and call readTimeSeriesValue
func CheckMetrics(MetricName string, startat time.Time) (float64, error) {
	ctx := context.Background()
	s, err := createService(ctx)
	if err != nil {
		logrus.Fatal(err)
		return 0, err
	}
	metricType := MetricName
	return readTimeSeriesValue(s, metricType, startat)

}

func readTimeSeriesValue(s *monitoring.Service, metricType string, startat time.Time, apiKey string, appKey string) (float64, error) {
	projectID, err := utils.ProjectName()
	if err != nil {
		logrus.Error(err)
		return 0, err
	}
	logrus.Debugf("readTimeSeriesValue for %s", metricType)
	startTime := time.Now().UTC().Add(time.Until(startat))
	endTime := time.Now().UTC()
	resp, err := s.Projects.TimeSeries.List(utils.ProjectResource(projectID)).
		Filter(fmt.Sprintf("metric.type=\"%s\"", metricType)).
		IntervalStartTime(startTime.Format(time.RFC3339Nano)).
		IntervalEndTime(endTime.Format(time.RFC3339Nano)).
		Do()
	if err != nil {
		logrus.Error(err)
		return 0.0, fmt.Errorf("Could not read time series value, %v ", err)
	}
	errSum := int64(0)
	if len(resp.TimeSeries) != 0 {
		for _, p := range resp.TimeSeries[0].Points {
			errSum = errSum + *p.Value.Int64Value
		}
		logrus.Debugf("errSum %d", errSum)
		return float64(errSum) / endTime.Sub(startTime).Seconds(), nil
	}
	return 0, nil
}

func createService(ctx context.Context) (*monitoring.Service, error) {
	hc, err := google.DefaultClient(ctx, monitoring.MonitoringScope)
	if err != nil {
		return nil, err
	}
	s, err := monitoring.New(hc)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return s, nil
}
