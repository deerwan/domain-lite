package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCloudflareListZones(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer testtoken" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/zones" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"result":  []map[string]string{{"id": "z1", "name": "example.com"}},
		})
	}))
	defer srv.Close()

	p := &cloudflareProvider{token: "testtoken", client: srv.Client(), base: srv.URL}
	zones, err := p.ListZones(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 1 || zones[0].Name != "example.com" {
		t.Fatalf("got %+v", zones)
	}
}

func TestCloudflareListRecords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"result": []map[string]interface{}{
				{"id": "r1", "name": "www.example.com", "type": "A", "content": "1.2.3.4", "ttl": 1},
			},
		})
	}))
	defer srv.Close()

	p := &cloudflareProvider{token: "t", client: srv.Client(), base: srv.URL}
	recs, err := p.ListRecords(context.Background(), "z1")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Content != "1.2.3.4" || recs[0].TTL != 1 {
		t.Fatalf("got %+v", recs)
	}
}

func TestCloudflareCreateRecord(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/zones/z1/dns_records" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"result":  map[string]string{"id": "newid"},
		})
	}))
	defer srv.Close()

	p := &cloudflareProvider{token: "t", client: srv.Client(), base: srv.URL}
	id, err := p.CreateRecord(context.Background(), "z1", Record{Name: "www", Type: "A", Content: "1.1.1.1"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "newid" {
		t.Fatalf("got id %q", id)
	}
}

func TestAliyunListZones(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("Action") != "DescribeDomains" {
			t.Errorf("action=%s", q.Get("Action"))
		}
		if q.Get("Signature") == "" {
			t.Error("missing signature")
		}
		if q.Get("AccessKeyId") != "akid" {
			t.Error("bad access key id")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Domains": map[string]interface{}{
				"Domain": []map[string]string{{"DomainName": "example.com"}},
			},
		})
	}))
	defer srv.Close()

	p := &aliyunProvider{accessKeyID: "akid", accessKeySecret: "aksec", client: srv.Client(), endpoint: srv.URL + "/"}
	zones, err := p.ListZones(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 1 || zones[0].Name != "example.com" {
		t.Fatalf("got %+v", zones)
	}
}

func TestAliyunListRecords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("Action") != "DescribeDomainRecords" || q.Get("DomainName") != "example.com" {
			t.Errorf("bad params action=%s domain=%s", q.Get("Action"), q.Get("DomainName"))
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"DomainRecords": map[string]interface{}{
				"Record": []map[string]interface{}{
					{"RecordId": "rec1", "RR": "www", "Type": "A", "Value": "1.2.3.4", "TTL": 600, "Priority": 0},
				},
			},
		})
	}))
	defer srv.Close()

	p := &aliyunProvider{accessKeyID: "akid", accessKeySecret: "aksec", client: srv.Client(), endpoint: srv.URL + "/"}
	recs, err := p.ListRecords(context.Background(), "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Content != "1.2.3.4" || recs[0].Name != "www" {
		t.Fatalf("got %+v", recs)
	}
}
