package linebot

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"golang.org/x/net/context"
)

func TestGetMessageContent(t *testing.T) {
	type want struct {
		URLPath         string
		RequestBody     []byte
		Response        *MessageContentResponse
		ResponseContent []byte
		Error           error
	}
	var testCases = []struct {
		MessageID      string
		ResponseCode   int
		Response       []byte
		ResponseHeader map[string]string
		Want           want
	}{
		{
			MessageID:      "325708",
			ResponseCode:   200,
			Response:       []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10},
			ResponseHeader: map[string]string{"Content-Disposition": `attachment; filename="example.jpg"`},
			Want: want{
				URLPath:     fmt.Sprintf(APIEndpointMessageContent, "325708"),
				RequestBody: []byte(""),
				Response: &MessageContentResponse{
					FileName: "example.jpg",
				},
				ResponseContent: []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10},
			},
		},
		{
			// 503 Service Unavailable
			MessageID:    "325708",
			ResponseCode: 503,
			Response:     []byte("Service Unavailable"),
			Want: want{
				URLPath:     fmt.Sprintf(APIEndpointMessageContent, "325708"),
				RequestBody: []byte(""),
				Error: &APIError{
					Code: 503,
				},
			},
		},
	}

	var currentTestIdx int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		tc := testCases[currentTestIdx]
		if r.Method != http.MethodGet {
			t.Errorf("Method %s; want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != tc.Want.URLPath {
			t.Errorf("URLPath %s; want %s", r.URL.Path, tc.Want.URLPath)
		}
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(body, tc.Want.RequestBody) {
			t.Errorf("RequestBody %s; want %s", body, tc.Want.RequestBody)
		}
		for k, v := range tc.ResponseHeader {
			w.Header().Add(k, v)
		}
		w.WriteHeader(tc.ResponseCode)
		w.Write(tc.Response)
	}))
	defer server.Close()
	client, err := mockClient(server)
	if err != nil {
		t.Error(err)
	}
	for i, tc := range testCases {
		currentTestIdx = i
		res, err := client.GetMessageContent(tc.MessageID).Do()
		if tc.Want.Error != nil {
			if !reflect.DeepEqual(err, tc.Want.Error) {
				t.Errorf("Error %d %q; want %q", i, err, tc.Want.Error)
			}
		} else {
			if err != nil {
				t.Error(err)
			}
		}
		if tc.Want.Response != nil {
			body := res.Content
			defer body.Close()
			res.Content = nil // Set nil because streams aren't comparable.
			if !reflect.DeepEqual(res, tc.Want.Response) {
				t.Errorf("Response %d %q; want %q", i, res, tc.Want.Response)
			}
			bodyGot, err := ioutil.ReadAll(body)
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(bodyGot, tc.Want.ResponseContent) {
				t.Errorf("ResponseContent %d %X; want %X", i, bodyGot, tc.Want.ResponseContent)
			}
		}
	}
}

func TestGetMessageContentWithContext(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		time.Sleep(10 * time.Millisecond)
		w.Header().Add("Content-Disposition", `attachment; filename="example.jpg"`)
		w.Write([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10})
	}))
	defer server.Close()
	client, err := mockClient(server)
	if err != nil {
		t.Error(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, err = client.GetMessageContent("325708A").WithContext(ctx).Do()
	if err != context.DeadlineExceeded {
		t.Errorf("err %v; want %v", err, context.Canceled)
	}
}

func BenchmarkGetMessageContent(b *testing.B) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.Header().Add("Content-Disposition", `attachment; filename="example.jpg"`)
		w.Write([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10})
	}))
	defer server.Close()
	client, err := mockClient(server)
	if err != nil {
		b.Error(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetMessageContent("325708A").Do()
	}
}
