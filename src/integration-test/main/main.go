package main

import (
	"flag"
	"pkg/libs/log"

	"integration-test/tcase"
)

func main() {
	sourcePort := flag.Int("sourcePort", 20001, "source redis port")
	targetPort := flag.Int("targetPort", 30001, "target redis port")

	log.SetLevel(log.LEVEL_INFO)

	log.Infof("run test starts")

	source, target := *sourcePort, *targetPort
	for _, tc := range tcase.CaseList {
		tc.SetInfo(source, target)

		if err := tc.Before(); err != nil {
			log.Panicf("run case %v before stage failed: %v", tc.Info(), err)
		}

		if err := tc.Run(); err != nil {
			log.Panicf("run case %v run stage failed: %v", tc.Info(), err)
		}

		if err := tc.Before(); err != nil {
			log.Panicf("run case %v after stage failed: %v", tc.Info(), err)
		}

		// +50 in different case
		source += 50
		target += 50
	}

	log.Infof("finish all test case")
}
