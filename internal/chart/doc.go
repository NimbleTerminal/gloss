// Package chart turns tables and chart documents into terminal charts. It is
// the only part of gloss that knows Flint (microsoft/flint-chart) and the
// ntcharts spec: documents come in as JSON, go through the flint-ntcharts
// compiler when they are Flint, and leave as a Spec that spec.Build draws.
package chart
