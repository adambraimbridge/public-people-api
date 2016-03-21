FROM alpine:3.3
ADD *.go /public-people-api/
ADD people/*.go /public-people-api/people/
RUN apk add --update bash \
  && apk --update add git bzr gcc \
  && apk --update add go \
  && export GOPATH=/gopath \
  && REPO_PATH="github.com/Financial-Times/public-people-api" \
  && mkdir -p $GOPATH/src/${REPO_PATH} \
  && cp -r public-people-api/* $GOPATH/src/${REPO_PATH} \
  && cd $GOPATH/src/${REPO_PATH} \
  && go get -t ./... \
  && cd $GOPATH/src/${REPO_PATH} \
  && go build  \
  && mv public-people-api /app \
  && apk del go git bzr \
  && rm -rf $GOPATH /var/cache/apk/*
CMD exec /app --neo-url=$NEO_URL --port=$APP_PORT --batchSize=$BATCH_SIZE --graphiteTCPAddress=$GRAPHITE_ADDRESS --graphitePrefix=$GRAPHITE_PREFIX --logMetrics=false --env=local

