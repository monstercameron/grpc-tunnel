package main

import "testing"

func TestParseRunnerGoModModulePath(parseT *testing.T) {
	parseModulePath, parseErr := parseRunnerGoModModulePath("module github.com/monstercameron/grpc-tunnel\n\ngo 1.25.0\n")
	if parseErr != nil {
		parseT.Fatalf("parseRunnerGoModModulePath() error = %v, want nil", parseErr)
	}
	if parseModulePath != "github.com/monstercameron/grpc-tunnel" {
		parseT.Fatalf("parseRunnerGoModModulePath() = %q, want %q", parseModulePath, "github.com/monstercameron/grpc-tunnel")
	}
}

func TestParseRunnerGoModModulePath_MissingModuleLine(parseT *testing.T) {
	_, parseErr := parseRunnerGoModModulePath("go 1.25.0\n")
	if parseErr == nil {
		parseT.Fatal("parseRunnerGoModModulePath() expected error, got nil")
	}
}

func TestNormalizeRunnerRepositoryURL(parseT *testing.T) {
	parseTests := []struct {
		parseName     string
		parseInput    string
		parseExpected string
	}{
		{
			parseName:     "https with git suffix",
			parseInput:    "https://github.com/monstercameron/grpc-tunnel.git",
			parseExpected: "https://github.com/monstercameron/grpc-tunnel",
		},
		{
			parseName:     "ssh url",
			parseInput:    "git@github.com:monstercameron/grpc-tunnel.git",
			parseExpected: "https://github.com/monstercameron/grpc-tunnel",
		},
		{
			parseName:     "already normalized",
			parseInput:    "https://github.com/monstercameron/grpc-tunnel",
			parseExpected: "https://github.com/monstercameron/grpc-tunnel",
		},
	}

	for _, parseTestCase := range parseTests {
		parseT.Run(parseTestCase.parseName, func(parseT2 *testing.T) {
			parseNormalizedURL := normalizeRunnerRepositoryURL(parseTestCase.parseInput)
			if parseNormalizedURL != parseTestCase.parseExpected {
				parseT2.Fatalf("normalizeRunnerRepositoryURL() = %q, want %q", parseNormalizedURL, parseTestCase.parseExpected)
			}
		})
	}
}
