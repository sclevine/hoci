package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"golang.org/x/xerrors"
	"log"
	"os"
	"os/exec"
	"reflect"
	"strings"
)

type Package struct {
	Name    string        `json:"name" dpkg:"binary:Package"`
	Version string        `json:"version" dpkg:"Version"`
	Arch    string        `json:"arch" dpkg:"Architecture"`
	Source  SourcePackage `json:"source"`
	Summary string        `json:"summary" dpkg:"binary:Summary"`
}

type SourcePackage struct {
	Name            string `json:"name" dpkg:"source:Package"`
	Version         string `json:"version" dpkg:"source:Version"`
	UpstreamVersion string `json:"upstreamVersion" dpkg:"source:Upstream-Version"`
}

func main() {
	var pkgs []Package
	logger := log.New(os.Stderr, "", log.LstdFlags)
	err := DPKG{Log: logger}.Metadata(&pkgs)
	if err != nil {
		logger.Fatal(err)
	}
	out, err := json.Marshal(pkgs)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Println(string(out))
}

type DPKG struct {
	Log *log.Logger
}

func (p DPKG) Present() bool {
	if _, err := exec.LookPath("dpkg-query"); err != nil {
		return true
	}
	return false
}

func (p DPKG) Metadata(pkgs interface{}) error {
	v := reflect.ValueOf(pkgs)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return xerrors.New("argument is not a pointer")
	}

	sliceV := v.Elem()
	elemT := sliceV.Type().Elem()
	if sliceV.Kind() != reflect.Slice {
		return xerrors.New("pointer does not reference slice")
	}
	fields, err := findStructFields(elemT)
	if err != nil {
		return err
	}

	var query string
	for _, f := range fields {
		query += `${` + f + `}\t`
	}
	query = query[:len(query)-1] + "n"

	cmdErr := &bytes.Buffer{}
	cmdOut := &bytes.Buffer{}
	cmd := exec.Command("dpkg-query", "-W", "-f="+query)
	cmd.Stderr = cmdErr
	cmd.Stdout = cmdOut
	if err := cmd.Run(); err != nil {
		p.Log.Print(cmdErr.String())
		return err
	}

	var in [][]string
	for s := bufio.NewScanner(cmdOut); s.Scan(); {
		in = append(in, strings.Split(s.Text(), "\t"))
	}

	sliceV.SetLen(0)
	for _, row := range in {
		v := reflect.New(elemT)

		left, err := setStructFields(v.Elem(), row)
		if err != nil {
			return err
		}
		if left > 0 {
			return xerrors.New("invalid struct tags")
		}
		sliceV.Set(reflect.Append(sliceV, v.Elem()))
	}

	return nil
}


func findStructFields(t reflect.Type) ([]string, error) {
	var out []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.Type.Kind() == reflect.Struct {
			sub, err := findStructFields(field.Type)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
			continue
		}
		tag := field.Tag.Get("dpkg")
		if tag == "" {
			continue
		}

		if field.Type.Kind() != reflect.String {
			return nil, xerrors.Errorf("not string: %s", field.Type)
		}
		out = append(out, tag)
	}
	return out, nil
}

func setStructFields(v reflect.Value, vals []string) (left int, err error) {
	val := 0
	for i := 0; i < v.NumField(); i++ {
		fieldV := v.Field(i)
		fieldT := v.Type().Field(i)

		if fieldV.Kind() == reflect.Struct {
			left, err := setStructFields(fieldV, vals[val:])
			if err != nil {
				return 0, err
			}
			val = len(vals)-left
			continue
		}
		tag := fieldT.Tag.Get("dpkg")
		if tag == "" {
			continue
		}
		if fieldV.Kind() != reflect.String {
			return 0, xerrors.Errorf("not string: %s", fieldT)
		}
		fieldV.Set(reflect.ValueOf(vals[val]))
		val++
	}
	return len(vals)-val, nil
}