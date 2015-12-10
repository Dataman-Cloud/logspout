package logger

import (
	log "github.com/cihub/seelog"
)

func init() {
	logger, err := log.LoggerFromConfigAsString(logConfig())
	if err == nil {
		log.ReplaceLogger(logger)
	} else {
		log.Error(err)
	}
}

func logConfig() string {
	return `<seelog type="asynctimer" asyncinterval="5000000" minlevel="debug" maxlevel="error">
                  <outputs formatid="main">
                   <!--<console/>-->
                      <buffered size="10000" flushperiod="1000">
    <rollingfile type="size" filename="/var/log/logspout/logspout.log" maxsize="5000000" maxrolls="30" />
                                  <!--<file path="/var/log/logspout/logspout.log"/>-->
                      </buffered>
                     </outputs>
                      <formats>
                      <format id="main" format="%Date(2006-01-02 15:04:05Z07:00) [%LEVEL] %Msg%n"/>
                      </formats>
                </seelog>`
}
