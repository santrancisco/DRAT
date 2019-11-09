package jobs

import (
	"errors"
	"fmt"
	"log"

	que "github.com/santrancisco/cque"
)

var (
	ErrImmediateReschedule = errors.New("reschedule ASAP")
	ErrDidNotReschedule    = errors.New("no need to reschedule, we are done")
)

// JobFunc should do a thing. Return either:
// nil => No error, move onto next job
// ErrImmediateReschedule => wrapper try it again immediately.
// ErrDidNotReschedule => wrapper will rely on queue lib to reschedule or retry.
// any other error => wrapper will rely on que to reschedule
type JobFunc func(logger *log.Logger, qc *que.Client, job *que.Job, appconfig map[string]interface{}) error

type JobFuncWrapper struct {
	QC        *que.Client
	Logger    *log.Logger
	F         JobFunc
	AppConfig map[string]interface{}
}

func (scw *JobFuncWrapper) Run(job *que.Job) error {
	for {
		err := scw.tryRun(job)
		switch err {
		case nil:
			// No error at all, everything is fine - finish and move onto next job
			return nil
		case ErrImmediateReschedule: // Rescheduling immediately to this worker (TODO: add counter and retries limit)
			scw.Logger.Printf("[DEBUG] RE-RUN IMMEDIATELY, RESTARTING... %s", job.Type)
			continue
		case ErrDidNotReschedule:
			scw.Logger.Printf("[DEBUG] A %s job throw a ErrDidNotReschedule error code and will be discarded", job.Type)
			return nil
		default:
			// Note: cque does not support reschedule at the moment so this will be logged into stderr and discard.
			scw.Logger.Printf("[DEBUG] FAILED WITH ERROR, RELY ON QUE TO RESCHEDULE %s : %s", job.Type, err)
			return err
		}
	}
}

func limitStringLength(s string, size int) string {
	if len(s) > size {
		return s[:size]
	}
	return s
}

// This job manages the tx, no one else should commit or rollback
func (jfw *JobFuncWrapper) tryRun(job *que.Job) error {
	jfw.Logger.Printf(limitStringLength(fmt.Sprintf("[DEBUG] START %s - %v ", job.Type, job.Args), 100))
	defer jfw.Logger.Printf(limitStringLength(fmt.Sprintf("[DEBUG] STOP %s - %v ", job.Type, job.Args), 100))

	err := jfw.F(jfw.Logger, jfw.QC, job, jfw.AppConfig)
	switch err {
	case nil:
	// continue, we commit later
	// case ErrImmediateReschedule:
	// 	err = tx.Commit()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	return ErrImmediateReschedule
	default:
		return err
	}
	return nil
}
