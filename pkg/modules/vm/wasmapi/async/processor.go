package async

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/pkg/errors"

	"github.com/machinefi/w3bstream/pkg/depends/conf/log"
	"github.com/machinefi/w3bstream/pkg/depends/kit/mq"
	"github.com/machinefi/w3bstream/pkg/depends/kit/sqlx"
	"github.com/machinefi/w3bstream/pkg/depends/x/contextx"
	"github.com/machinefi/w3bstream/pkg/models"
	"github.com/machinefi/w3bstream/pkg/modules/event"
	apitypes "github.com/machinefi/w3bstream/pkg/modules/vm/wasmapi/types"
	"github.com/machinefi/w3bstream/pkg/types"
	"github.com/machinefi/w3bstream/pkg/types/wasm"
	"github.com/machinefi/w3bstream/pkg/types/wasm/kvdb"
)

type ApiCallProcessor struct {
	l      log.Logger
	router *gin.Engine
	cli    *asynq.Client
}

func NewApiCallProcessor(l log.Logger, router *gin.Engine, cli *asynq.Client) *ApiCallProcessor {
	return &ApiCallProcessor{
		l:      l,
		router: router,
		cli:    cli,
	}
}

func (p *ApiCallProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	payload := apiCallPayload{}
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	apiReq := apitypes.HttpRequest{}
	if err := json.Unmarshal(payload.Data, &apiReq); err != nil {
		return fmt.Errorf("http.ReadRequest failed: %v: %w", err, asynq.SkipRetry)
	}
	req, err := http.NewRequest(apiReq.Method, apiReq.Url, bytes.NewReader(apiReq.Body))
	if err != nil {
		return fmt.Errorf("http.ReadRequest failed: %v: %w", err, asynq.SkipRetry)
	}
	req.Header = apiReq.Header

	req = req.WithContext(contextx.WithContextCompose(
		types.WithProjectContext(payload.Project),
		wasm.WithChainClientContext(payload.ChainClient),
		types.WithLoggerContext(p.l),
	)(ctx))

	respRecorder := httptest.NewRecorder()
	p.router.ServeHTTP(respRecorder, req)

	projectName := payload.Project.ProjectName.Name
	_, l := p.l.Start(ctx, "wasmapi.ProcessTaskApiCall")
	defer l.End()
	l = l.WithValues("ProjectName", projectName)

	resp := respRecorder.Result()
	var body []byte
	if resp.Body != nil {
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			l.Error(errors.Wrap(err, "encode http response failed"))
			return fmt.Errorf("encode http response failed: %v: %w", err, asynq.SkipRetry)
		}
	}

	respHeader := resp.Header
	for k, v := range apiReq.Header {
		if k == "Content-Type" {
			continue
		}
		respHeader[k] = v
	}

	apiResp := apitypes.HttpResponse{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Proto:      resp.Proto,
		Header:     respHeader,
		Body:       body,
	}
	apiRespJson, err := json.Marshal(&apiResp)
	if err != nil {
		l.Error(errors.Wrap(err, "encode http response failed"))
		return fmt.Errorf("encode http response failed: %v: %w", err, asynq.SkipRetry)
	}

	eventType := req.Header.Get("eventType")
	if eventType == "" {
		l.Error(errors.New("miss eventType"))
		return fmt.Errorf("miss eventType, projectName %v: %w", projectName, asynq.SkipRetry)
	}

	task, err := newApiResultTask(projectName, eventType, apiRespJson)
	if err != nil {
		l.Error(errors.Wrap(err, "new api result task failed"))
		return fmt.Errorf("new api result task failed: %v: %w", err, asynq.SkipRetry)
	}
	if _, err := p.cli.Enqueue(task); err != nil {
		l.Error(errors.Wrap(err, "could not enqueue task"))
		return fmt.Errorf("could not enqueue task: %v: %w", err, asynq.SkipRetry)
	}

	return nil
}

type ApiResultProcessor struct {
	l     log.Logger
	mgrDB sqlx.DBExecutor
	kv    *kvdb.RedisDB
	tb    *mq.TaskBoard
	tw    *mq.TaskWorker
}

func NewApiResultProcessor(l log.Logger, mgrDB sqlx.DBExecutor, kv *kvdb.RedisDB, tb *mq.TaskBoard, tw *mq.TaskWorker) *ApiResultProcessor {
	return &ApiResultProcessor{
		l:     l,
		kv:    kv,
		mgrDB: mgrDB,
		tb:    tb,
		tw:    tw,
	}
}

func (p *ApiResultProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	payload := apiResultPayload{}
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	ctx = contextx.WithContextCompose(
		types.WithTaskBoardContext(p.tb),
		types.WithTaskWorkerContext(p.tw),
		types.WithLoggerContext(p.l),
		types.WithMgrDBExecutorContext(p.mgrDB),
		kvdb.WithRedisDBKeyContext(p.kv),
		types.WithProjectContext(&models.Project{
			ProjectName: models.ProjectName{Name: payload.ProjectName}},
		),
	)(ctx)

	_, l := p.l.Start(ctx, "wasmapi.ProcessTaskApiResult")
	defer l.End()

	if _, err := event.HandleEvent(ctx, payload.EventType, payload.Data); err != nil {
		l.Error(errors.Wrap(err, "send event failed"))
		return err
	}

	return nil
}
