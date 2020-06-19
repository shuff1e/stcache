package main

import (
	"context"
	"gopkg.in/natefinch/lumberjack.v2"
	"log"
)

type commonVerifier struct {
	verificationScript string
	context context.Context
	Logger *log.Logger
	inputCh chan verifyInput
	outputCh chan verifyOutput
	detector *Detector
}

func NewCommonVerifier(opts *options, logOutput *lumberjack.Logger) *commonVerifier{
	veri := &commonVerifier{verificationScript:opts.verificationScript}
	veri.detector = NewDetector(5,3)
	veri.Logger = log.New(logOutput,"verification : ",log.Ldate|log.Ltime)
	veri.detector.Logger = veri.Logger
	return veri
}

func (cv *commonVerifier) verify() {
	pool := New(100)
	for {
		select {
		case input := <-cv.inputCh:
			pool.Add(1)
			go func(input verifyInput) {
				defer pool.Done()
				cv.detector.Ping(cv.verificationScript,input)
				key := input.key
				value := input.value
				cv.outputCh <- verifyOutput{key,value, cv.detector.Phi(key)}
			}(input)
		case <-cv.context.Done():
			pool.Wait()
			return
		}
	}
}
