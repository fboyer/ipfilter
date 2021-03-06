package ipfilter

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mholt/caddy/middleware"
	"github.com/oschwald/maxminddb-golang"
)

const (
	// 'GeoLite2.mmdb' taken from 'MaxMind.com'
	// 'https://dev.maxmind.com/geoip/geoip2/geolite2/'
	DataBase = "./testdata/GeoLite2.mmdb"
	Page     = "./testdata/blockpage.html"
	Allow    = "allow"
	Block    = "block"
	BlockMsg = "You are not allowed here"
)

func TestCountryCodes(t *testing.T) {
	hasCountryCodes = true

	TestCases := []struct {
		ipfconf        IPFConfig
		reqIP          string
		scope          string
		expectedBody   string
		expectedStatus int
	}{
		{IPFConfig{
			PathScopes:   []string{"/"},
			BlockPage:    Page,
			Rule:         Allow,
			CountryCodes: []string{"JP", "SA"},
		},
			"8.8.8.8:_", // US
			"/",
			BlockMsg,
			http.StatusOK,
		},

		{IPFConfig{
			PathScopes:   []string{"/private"},
			BlockPage:    Page,
			Rule:         Block,
			CountryCodes: []string{"US", "CA"},
		},
			"24.53.192.20:_", // CA
			"/private",
			BlockMsg,
			http.StatusOK,
		},

		{IPFConfig{
			PathScopes:   []string{"/testdata"},
			Rule:         Block,
			CountryCodes: []string{"RU", "CN"},
		},
			"42.48.120.7:_", // CN
			"/",
			"",
			http.StatusOK, // pass-thru, out of scope
		},

		{IPFConfig{
			PathScopes:   []string{"/"},
			Rule:         Block,
			CountryCodes: []string{"RU", "JP", "SA"},
		},
			"78.95.221.163:_", // SA
			"/",
			"",
			http.StatusForbidden,
		},

		{IPFConfig{
			PathScopes:   []string{"/onlyus"},
			Rule:         Allow,
			CountryCodes: []string{"US"},
		},
			"5.175.96.22:_", // RU
			"/onlyus",
			"",
			http.StatusForbidden,
		},

		{IPFConfig{
			PathScopes:   []string{"/"},
			Rule:         Allow,
			CountryCodes: []string{"FR", "GB", "AE", "DE"},
		},
			"5.4.9.3:_", // DE
			"/",
			"",
			http.StatusOK, // Allowed
		},
	}
	// open the db
	db, err := maxminddb.Open(DataBase)
	if err != nil {
		t.Fatalf("Error opening the database: %v", err)
	}
	defer db.Close()

	for _, tc := range TestCases {
		if tc.ipfconf.Rule == Block {
			isBlock = true
		} else {
			isBlock = false
		}

		ipf := IPFilter{
			Next: middleware.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
				return http.StatusOK, nil
			}),
			Config: tc.ipfconf,
		}

		// set the DBHandler
		ipf.Config.DBHandler = db

		req, err := http.NewRequest("GET", tc.scope, nil)
		if err != nil {
			t.Fatalf("Could not create HTTP request: %v", err)
		}

		req.RemoteAddr = tc.reqIP

		rec := httptest.NewRecorder()

		status, _ := ipf.ServeHTTP(rec, req)
		if status != tc.expectedStatus {
			t.Fatalf("Expected StatusCode: '%d', Got: '%d'\nTestCase: %v\n",
				tc.expectedStatus, status, tc)
		}

		if rec.Body.String() != tc.expectedBody {
			t.Fatalf("Expected Body: '%s', Got: '%s'\nTestCase: %v\n",
				tc.expectedBody, rec.Body.String(), tc)
		}
	}
	// get ready for the next test
	hasCountryCodes = false
}

func TestRanges(t *testing.T) {
	hasRanges = true

	TestCases := []struct {
		ipfconf        IPFConfig
		reqIP          string
		scope          string
		expectedBody   string
		expectedStatus int
	}{
		{IPFConfig{
			PathScopes: []string{"/"},
			BlockPage:  Page,
			Rule:       Block,
			Ranges: []Range{
				Range{
					net.ParseIP("243.1.3.10"),
					net.ParseIP("243.1.3.20"),
				},
			},
		},
			"243.1.3.15:_",
			"/",
			BlockMsg,
			http.StatusOK,
		},

		{IPFConfig{
			PathScopes: []string{"/private"},
			BlockPage:  Page,
			Rule:       Block,
			Ranges: []Range{
				Range{
					net.ParseIP("243.1.3.10"),
					net.ParseIP("243.1.3.20"),
				},
				Range{
					net.ParseIP("202.33.44.1"),
					net.ParseIP("202.33.44.255"),
				},
			},
		},
			"202.33.44.224:_",
			"/private",
			BlockMsg,
			http.StatusOK,
		},

		{IPFConfig{
			PathScopes: []string{"/"},
			BlockPage:  Page,
			Rule:       Block,
			Ranges: []Range{
				Range{
					net.ParseIP("243.1.3.10"),
					net.ParseIP("243.1.3.20"),
				},
			},
		},
			"243.1.3.9:_",
			"/",
			"",
			http.StatusOK,
		},

		{IPFConfig{
			PathScopes: []string{"/eighties"},
			BlockPage:  Page,
			Rule:       Allow,
			Ranges: []Range{
				Range{
					net.ParseIP("243.1.3.10"),
					net.ParseIP("243.1.3.20"),
				},
				Range{
					net.ParseIP("80.0.0.0"),
					net.ParseIP("80.255.255.255"),
				},
			},
		},
			"80.245.155.250:_",
			"/eighties",
			"",
			http.StatusOK,
		},

		{IPFConfig{
			PathScopes: []string{"/eighties"},
			Rule:       Block,
			Ranges: []Range{
				Range{
					net.ParseIP("243.1.3.10"),
					net.ParseIP("243.1.3.20"),
				},
				Range{
					net.ParseIP("80.0.0.0"),
					net.ParseIP("80.255.255.255"),
				},
			},
		},
			"80.245.155.250:_",
			"/",
			"",
			http.StatusOK,
		},

		{IPFConfig{
			PathScopes: []string{"/"},
			Rule:       Block,
			Ranges: []Range{
				Range{
					net.ParseIP("243.1.3.10"),
					net.ParseIP("243.1.3.20"),
				},
				Range{
					net.ParseIP("80.0.0.0"),
					net.ParseIP("80.255.255.255"),
				},
				Range{
					net.ParseIP("23.1.3.1"),
					net.ParseIP("23.1.3.20"),
				},
				Range{
					net.ParseIP("85.0.0.0"),
					net.ParseIP("85.255.255.255"),
				},
			},
		},
			"23.1.3.9:_",
			"/",
			"",
			http.StatusForbidden,
		},
		// From here on out, tests are covering single IP ranges
		{IPFConfig{
			PathScopes: []string{"/"},
			BlockPage:  Page,
			Rule:       Block,
			Ranges: []Range{
				Range{
					net.ParseIP("8.8.8.8"),
					net.ParseIP("8.8.8.8"),
				},
			},
		},
			"8.8.4.4:_",
			"/",
			"",
			http.StatusOK,
		},

		{IPFConfig{
			PathScopes: []string{"/"},
			BlockPage:  Page,
			Rule:       Allow,
			Ranges: []Range{
				Range{
					net.ParseIP("8.8.8.8"),
					net.ParseIP("8.8.8.8"),
				},
			},
		},
			"8.8.4.4:_",
			"/",
			BlockMsg,
			http.StatusOK,
		},

		{IPFConfig{
			PathScopes: []string{"/private"},
			BlockPage:  Page,
			Rule:       Allow,
			Ranges: []Range{
				Range{
					net.ParseIP("52.9.1.2"),
					net.ParseIP("52.9.1.2"),
				},
				Range{
					net.ParseIP("52.9.1.3"),
					net.ParseIP("52.9.1.3"),
				},
				Range{
					net.ParseIP("52.9.1.4"),
					net.ParseIP("52.9.1.4"),
				},
			},
		},
			"52.9.1.3:_",
			"/private",
			"",
			http.StatusOK,
		},

		{IPFConfig{
			PathScopes: []string{"/private"},
			BlockPage:  Page,
			Rule:       Allow,
			Ranges: []Range{
				Range{
					net.ParseIP("99.1.8.8"),
					net.ParseIP("99.1.8.8"),
				},
			},
		},
			"90.90.90.90:_",
			"/",
			"",
			http.StatusOK,
		},

		{IPFConfig{
			PathScopes: []string{"/private"},
			Rule:       Block,
			Ranges: []Range{
				Range{
					net.ParseIP("52.9.1.2"),
					net.ParseIP("52.9.1.2"),
				},
				Range{
					net.ParseIP("52.9.1.3"),
					net.ParseIP("52.9.1.3"),
				},
				Range{
					net.ParseIP("52.9.1.4"),
					net.ParseIP("52.9.1.4"),
				},
			},
		},
			"52.9.1.3:_",
			"/private",
			"",
			http.StatusForbidden,
		},
	}

	for _, tc := range TestCases {
		if tc.ipfconf.Rule == Block {
			isBlock = true
		} else {
			isBlock = false
		}

		ipf := IPFilter{
			Next: middleware.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
				return http.StatusOK, nil
			}),
			Config: tc.ipfconf,
		}
		req, err := http.NewRequest("GET", tc.scope, nil)
		if err != nil {
			t.Fatalf("Could not create HTTP request: %v", err)
		}

		req.RemoteAddr = tc.reqIP

		rec := httptest.NewRecorder()

		status, _ := ipf.ServeHTTP(rec, req)
		if status != tc.expectedStatus {
			t.Fatalf("Expected StatusCode: '%d', Got: '%d'\nTestCase: %v\n",
				tc.expectedStatus, status, tc)
		}

		if rec.Body.String() != tc.expectedBody {
			t.Fatalf("Expected Body: '%s', Got: '%s'\nTestCase: %v\n",
				tc.expectedBody, rec.Body.String(), tc)
		}
	}
}

func TestFwdForIPs(t *testing.T) {
	// These test cases provide test coverage for proxied requests support (Refer to https://github.com/pyed/ipfilter/pull/4)
	TestCases := []struct {
		ipfconf        IPFConfig
		reqIP          string
		fwdFor         string
		scope          string
		expectedStatus int
	}{
		// Middleware should block request when filtering rule is set to 'Block', a *blocked* IP is passed in the 'X-Forwarded-For' header and the request is coming from *permitted* remote address
		{
			IPFConfig{
				PathScopes: []string{"/"},
				Rule:       Block,
				Ranges: []Range{
					Range{
						net.ParseIP("8.8.8.8"),
						net.ParseIP("8.8.8.8"),
					},
				},
			},
			"8.8.4.4:12345",
			"8.8.8.8",
			"/",
			http.StatusForbidden,
		},
		// Middleware should allow request when filtering rule is set to 'Block', no IP is passed in the 'X-Forwarded-For' header and the request is coming from *permitted* remote address
		{
			IPFConfig{
				PathScopes: []string{"/"},
				Rule:       Block,
				Ranges: []Range{
					Range{
						net.ParseIP("8.8.8.8"),
						net.ParseIP("8.8.8.8"),
					},
				},
			},
			"8.8.4.4:12345",
			"",
			"/",
			http.StatusOK,
		},
		// Middleware should allow request when filtering rule is set to 'Block', a *permitted* IP is passed in the 'X-Forwarded-For' header and the request is coming from *blocked* remote address
		{
			IPFConfig{
				PathScopes: []string{"/"},
				Rule:       Block,
				Ranges: []Range{
					Range{
						net.ParseIP("8.8.8.8"),
						net.ParseIP("8.8.8.8"),
					},
				},
			},
			"8.8.8.8:12345",
			"8.8.4.4",
			"/",
			http.StatusOK,
		},
		// Middleware should allow request when filtering rule is set to 'Allow', a *permitted* IP is passed in the 'X-Forwarded-For' header and the request is coming from *blocked* remote address
		{
			IPFConfig{
				PathScopes: []string{"/"},
				Rule:       Allow,
				Ranges: []Range{
					Range{
						net.ParseIP("8.8.8.8"),
						net.ParseIP("8.8.8.8"),
					},
				},
			},
			"8.8.4.4:12345",
			"8.8.8.8",
			"/",
			http.StatusOK,
		},
		// Middleware should block request when filtering rule is set to 'Allow', no IP is passed in the 'X-Forwarded-For' header and the request is coming from *blocked* remote address
		{
			IPFConfig{
				PathScopes: []string{"/"},
				Rule:       Allow,
				Ranges: []Range{
					Range{
						net.ParseIP("8.8.8.8"),
						net.ParseIP("8.8.8.8"),
					},
				},
			},
			"8.8.4.4:12345",
			"",
			"/",
			http.StatusForbidden,
		},
		// Middleware should block request when filtering rule is set to 'Allow', a *blocked* IP is passed in the 'X-Forwarded-For' header and the request is coming from *permitted* remote address
		{
			IPFConfig{
				PathScopes: []string{"/"},
				Rule:       Allow,
				Ranges: []Range{
					Range{
						net.ParseIP("8.8.8.8"),
						net.ParseIP("8.8.8.8"),
					},
				},
			},
			"8.8.8.8:12345",
			"8.8.4.4",
			"/",
			http.StatusForbidden,
		},
	}

	for _, tc := range TestCases {
		if tc.ipfconf.Rule == Block {
			isBlock = true
		} else {
			isBlock = false
		}

		ipf := IPFilter{
			Next: middleware.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
				return http.StatusOK, nil
			}),
			Config: tc.ipfconf,
		}

		req, err := http.NewRequest("GET", tc.scope, nil)
		if err != nil {
			t.Fatalf("Could not create HTTP request: %v", err)
		}

		req.RemoteAddr = tc.reqIP
		if tc.fwdFor != "" {
			req.Header.Set("X-Forwarded-For", tc.fwdFor)
		}

		rec := httptest.NewRecorder()

		status, _ := ipf.ServeHTTP(rec, req)
		if status != tc.expectedStatus {
			t.Fatalf("Expected StatusCode: '%d', Got: '%d'\nTestCase: %v\n",
				tc.expectedStatus, status, tc)
		}
	}
}
