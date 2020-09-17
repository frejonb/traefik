package metrics

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/go-kit/kit/metrics"
	"github.com/stretchr/testify/assert"
)

// CollectingCounter is a metrics.Counter implementation that enables access to the CounterValue and LastLabelValues.
type CollectingCounter struct {
	CounterValue    float64
	LastLabelValues []string
}

// With is there to satisfy the metrics.Counter interface.
func (c *CollectingCounter) With(labelValues ...string) metrics.Counter {
	c.LastLabelValues = labelValues
	return c
}

// Add is there to satisfy the metrics.Counter interface.
func (c *CollectingCounter) Add(delta float64) {
	c.CounterValue += delta
}

func TestMetricsRetryListener(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	retryMetrics := newCollectingRetryMetrics()
	retryListener := NewRetryListener(retryMetrics, "serviceName")
	retryListener.Retried(req, 1)
	retryListener.Retried(req, 2)

	wantCounterValue := float64(2)
	if retryMetrics.retriesCounter.CounterValue != wantCounterValue {
		t.Errorf("got counter value of %f, want %f", retryMetrics.retriesCounter.CounterValue, wantCounterValue)
	}

	wantLabelValues := []string{"service", "serviceName"}
	if !reflect.DeepEqual(retryMetrics.retriesCounter.LastLabelValues, wantLabelValues) {
		t.Errorf("wrong label values %v used, want %v", retryMetrics.retriesCounter.LastLabelValues, wantLabelValues)
	}
}

// collectingRetryMetrics is an implementation of the retryMetrics interface that can be used inside tests to collect the times Add() was called.
type collectingRetryMetrics struct {
	retriesCounter *CollectingCounter
}

func newCollectingRetryMetrics() *collectingRetryMetrics {
	return &collectingRetryMetrics{retriesCounter: &CollectingCounter{}}
}

func (m *collectingRetryMetrics) ServiceRetriesCounter() metrics.Counter {
	return m.retriesCounter
}

type rwWithCloseNotify struct {
	*httptest.ResponseRecorder
}

func (r *rwWithCloseNotify) CloseNotify() <-chan bool {
	panic("implement me")
}

func TestCloseNotifier(t *testing.T) {
	testCases := []struct {
		rw                      http.ResponseWriter
		desc                    string
		implementsCloseNotifier bool
	}{
		{
			rw:                      httptest.NewRecorder(),
			desc:                    "does not implement CloseNotifier",
			implementsCloseNotifier: false,
		},
		{
			rw:                      &rwWithCloseNotify{httptest.NewRecorder()},
			desc:                    "implements CloseNotifier",
			implementsCloseNotifier: true,
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			_, ok := test.rw.(http.CloseNotifier)
			assert.Equal(t, test.implementsCloseNotifier, ok)

			rw := newResponseRecorder(test.rw)
			_, impl := rw.(http.CloseNotifier)
			assert.Equal(t, test.implementsCloseNotifier, impl)
		})
	}
}

func TestGetHost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://foo.bar/baz", nil)
	host := getHost(req)

	wantHost := "foo.bar"
	if host != wantHost {
		t.Errorf("got host value of %s, want %s", host, wantHost)
	}
}

func TestKeepMetric(t *testing.T) {
	host1 := "foo.bar"
	host2 := "api.bar"
	toKeep1 := keepMetric(host1)
	toKeep2 := keepMetric(host2)

	wantToKeep1 := false
	if toKeep1 != wantToKeep1 {
		t.Errorf("got keep metric value of %t, want %t", toKeep1, wantToKeep1)
	}
	wantToKeep2 := true
	if toKeep2 != wantToKeep2 {
		t.Errorf("got keep metric value of %t, want %t", toKeep2, wantToKeep2)
	}
}

func TestGetPath(t *testing.T) {
	testCases := []struct {
		desc     string
		req      *http.Request
		wantPath string
	}{
		{
			desc:     "Empty path.",
			req:      httptest.NewRequest(http.MethodGet, "https://foo.bar", nil),
			wantPath: "undefined",
		},
		{
			desc:     "Path does not match regex should be left alone.",
			req:      httptest.NewRequest(http.MethodGet, "https://foo.bar/v1.2/baz/fizz/buzz", nil),
			wantPath: "/v1.2/baz/fizz/buzz",
		},
		{
			desc:     "Path that matches regex should be modified.",
			req:      httptest.NewRequest(http.MethodGet, "https://foo.bar/v1.2/service/foo", nil),
			wantPath: "/v1.2/service/foo",
		},
		{
			desc:     "Long path that matches regex should be modified.",
			req:      httptest.NewRequest(http.MethodGet, "https://foo.bar/v1.2/service/foo/bar/baz", nil),
			wantPath: "/v1.2/service/foo",
		},
		{
			desc:     "Path with params should not include params.",
			req:      httptest.NewRequest(http.MethodGet, "https://foo.bar/v1.2/service/foo?bar=baz", nil),
			wantPath: "/v1.2/service/foo",
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			ok := getPath(test.req)
			assert.Equal(t, test.wantPath, ok)
		})
	}
}
