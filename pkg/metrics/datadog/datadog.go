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

package datadog

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/zorkian/go-datadog-api"
)

func CheckMetrics(MetricName string, startat time.Time, apiKey string, appKey string) (float64, error) {
	client := datadog.NewClient(apiKey, appKey)
	startTime := time.Now().UTC().Add(time.Until(startat))
	endTime := time.Now().UTC()

	resp, err := client.QueryMetrics(startTime.Unix(), endTime.Unix(), MetricName)
	if err != nil {
		logrus.Error(err)
		return 0, err
	}
	if len(resp) == 0 {
		return 0, nil
	}
	errSum := float64(0)
	for _, p := range resp[0].Points {
		errSum = errSum + *p[1]
	}
	logrus.Debugf("errSum %d", errSum)
	return float64(errSum) / endTime.Sub(startTime).Seconds(), nil

}
