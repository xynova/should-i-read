package mailreport

import stroplog "github.com/behaviorengineering/strop/pkg/log"

// discardLogger satisfies stroplog.Logger for headless classify runs.
type discardLogger struct{}

func (d discardLogger) WithField(_ string, _ interface{}) stroplog.Logger   { return d }
func (d discardLogger) WithFields(_ map[string]interface{}) stroplog.Logger { return d }
func (d discardLogger) WithError(_ error) stroplog.Logger                   { return d }
func (d discardLogger) Debug(_ ...interface{})                              {}
func (d discardLogger) Info(_ ...interface{})                               {}
func (d discardLogger) Warn(_ ...interface{})                               {}
func (d discardLogger) Error(_ ...interface{})                              {}
