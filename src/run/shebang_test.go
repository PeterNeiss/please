package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShebangShell(t *testing.T) {
	cases := []struct {
		script string
		applet string
		ok     bool
	}{
		{"#!/bin/bash\n\nwc -w $@\n", "bash", true},
		{"#!/bin/sh\r\necho hi\r\n", "sh", true},
		{"#! /bin/ash\n", "sh", true},
		{"#!/usr/bin/env bash\n", "bash", true},
		{"#!/usr/bin/env -S bash -e\n", "bash", true},
		{`#!C:\tools\bash.exe` + "\n", "", false},
		{"#!/usr/bin/env python3\n", "", false},
		{"#!/usr/bin/python\n", "", false},
		{"#!/usr/bin/env\n", "", false},
		{"#!\n", "", false},
		{"echo no shebang\n", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		applet, ok := shebangShell([]byte(tc.script))
		assert.Equal(t, tc.ok, ok, "%q", tc.script)
		assert.Equal(t, tc.applet, applet, "%q", tc.script)
	}
}
