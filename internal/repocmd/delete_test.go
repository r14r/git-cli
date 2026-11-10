package repocmd

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type fakeDeleteClient struct {
	checkStatuses []int
	checkErrors   []error
	deletionError error
	deletionCalls int
	lastRepo      string
	lastYes       bool
}

func (f *fakeDeleteClient) check(_ string) (int, error) {
	status, err := f.checkStatuses[0], f.checkErrors[0]
	f.checkStatuses = f.checkStatuses[1:]
	f.checkErrors = f.checkErrors[1:]
	return status, err
}

func (f *fakeDeleteClient) delete(repo string, yes bool, _, _ io.Writer) error {
	f.deletionCalls++
	f.lastRepo, f.lastYes = repo, yes
	return f.deletionError
}

func TestDeleteWorkflow(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		statuses       []int
		apiErrors      []error
		deleteErr      error
		exit           int
		calls          int
		repo           string
		yes            bool
		stdoutContains []string
	}{
		{"short name", []string{"test", "--yes"}, []int{200, 404}, []error{nil, errors.New("404")}, nil, 0, 1, "r14r/test", true, []string{"1. Checking Repo:     EXISTS", "2. Deleting Repo:     DELETING", "3. Checking Repo:     DELETED"}},
		{"full name", []string{"other/repo"}, []int{200, 404}, []error{nil, errors.New("404")}, nil, 0, 1, "other/repo", false, []string{"3. Checking Repo:     DELETED"}},
		{"missing", []string{"test", "--yes"}, []int{404}, []error{errors.New("404")}, nil, 1, 0, "", false, []string{"NOT FOUND (or inaccessible)", "SKIPPED"}},
		{"auth error", []string{"test", "--yes"}, []int{403}, []error{errors.New("forbidden")}, nil, 1, 0, "", false, []string{"1. Checking Repo:     ERROR"}},
		{"network error", []string{"test", "--yes"}, []int{0}, []error{errors.New("offline")}, nil, 1, 0, "", false, []string{"1. Checking Repo:     ERROR"}},
		{"delete failed", []string{"test", "--yes"}, []int{200, 200}, []error{nil, nil}, errors.New("delete denied"), 1, 1, "r14r/test", true, []string{"2. Deleting Repo:     FAILED", "3. Checking Repo:     STILL EXISTS"}},
		{"verification failed", []string{"test", "--yes"}, []int{200, 0}, []error{nil, errors.New("network")}, nil, 1, 1, "r14r/test", true, []string{"3. Checking Repo:     ERROR"}},
		{"still exists", []string{"test", "--yes"}, []int{200, 200}, []error{nil, nil}, nil, 1, 1, "r14r/test", true, []string{"3. Checking Repo:     STILL EXISTS"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeDeleteClient{checkStatuses: tt.statuses, checkErrors: tt.apiErrors, deletionError: tt.deleteErr}
			var out, errOut bytes.Buffer
			code := runDeleteWithClient(tt.args, &out, &errOut, client)
			if code != tt.exit || client.deletionCalls != tt.calls {
				t.Fatalf("exit=%d calls=%d stdout=%s stderr=%s", code, client.deletionCalls, out.String(), errOut.String())
			}
			if tt.calls != 0 && (client.lastRepo != tt.repo || client.lastYes != tt.yes) {
				t.Fatalf("delete called with repo=%s yes=%v", client.lastRepo, client.lastYes)
			}
			for _, fragment := range tt.stdoutContains {
				if !strings.Contains(out.String(), fragment) {
					t.Errorf("missing %q in %q", fragment, out.String())
				}
			}
			if code == 1 && strings.Contains(out.String(), "3. Checking Repo:     DELETED") {
				t.Fatal("false success reported")
			}
		})
	}
}

func TestDeleteRejectsInvalidInput(t *testing.T) {
	for _, args := range [][]string{{}, {"--bad"}, {"test", "other"}, {"https://github.com/r14r/test", "--yes"}, {"../test", "--yes"}, {"test", "--yes", "--no"}} {
		var out, errOut bytes.Buffer
		client := &fakeDeleteClient{}
		if code := runDeleteWithClient(args, &out, &errOut, client); code != 2 || client.deletionCalls != 0 {
			t.Fatalf("args=%v code=%d err=%s", args, code, errOut.String())
		}
	}
}

func TestDeleteHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runDeleteWithClient([]string{"--help"}, &out, &errOut, &fakeDeleteClient{}); code != 0 {
		t.Fatalf("help code=%d", code)
	}
	if !strings.Contains(out.String(), "git-cli repo delete") {
		t.Fatal("missing help")
	}
}

func TestParseHTTPStatus(t *testing.T) {
	for _, tt := range []struct {
		raw  string
		want int
	}{
		{"HTTP/2 200 OK\r\nHeader: value\r\n", 200},
		{"HTTP/1.1 404 Not Found\r\n", 404},
		{"gh: connection failed", 0},
		{"HTTP/2 403 Forbidden\n", 403},
	} {
		if got := parseHTTPStatus([]byte(tt.raw)); got != tt.want {
			t.Errorf("parse(%q)=%d want %d", tt.raw, got, tt.want)
		}
	}
}
