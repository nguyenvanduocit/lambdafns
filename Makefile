.PHONY: deps clean build

deps:
	go get -u ./...

init: deps create-bucket

clean: 
	rm -rf ./crawl-rss/crawl-rss

build: clean
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o crawl-rss/crawl-rss ./crawl-rss

create-bucket:
	aws s3 mb --profile 12bit s3://12bitlambdafns

delete-bucket:
	aws s3 rm --profile 12bit s3://12bitlambdafns

package: build
	sam package --profile 12bit --template-file template.yaml --s3-bucket 12bitlambdafns --output-template-file packaged.yaml

deploy: package
	sam deploy --profile 12bit --template-file ./packaged.yaml --stack-name a12bitlambdafns --capabilities CAPABILITY_IAM

delete: delete-bucket
	aws cloudformation delete-stack --profile 12bit --stack-name a12bitlambdafns

start: build
	sam local start-api