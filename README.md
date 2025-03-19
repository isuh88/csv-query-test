# CSV Query Service

## 목차
- [프로젝트 구조](#프로젝트-구조)
- [주요 기능](#주요-기능)
  - [CSV 파일 업로드](#1-csv-파일-업로드)
  - [CSV 파일 조회](#2-csv-파일-조회)
- [API 엔드포인트](#api-엔드포인트)
- [성능 측정](#성능-측정)
- [설정](#설정)
- [실행 방법](#실행-방법)
- [사용 예시](#사용-예시)

이 프로젝트는 대용량 CSV 파일을 효율적으로 업로드하고 조회하기 위한 HTTP 서비스입니다. 다양한 업로드 전략을 지원하며, 세그먼트 단위로 파일을 관리합니다.

## 프로젝트 구조

```
.
├── main.go           # 서버 진입점 및 라우팅 설정
├── handler.go        # 조회 핸들러 구현
├── upload_handler.go # 업로드 핸들러 구현
├── s3_client.go      # S3 클라이언트
└── time_check.go    # 성능 측정 유틸리티
```

## 주요 기능

### 1. CSV 파일 업로드
4가지 다른 업로드 전략을 제공합니다:

1. **Fine-grained 업로드** (기본)
   - 세그먼트 단위로 즉시 업로드
   - 메모리 사용량 최소화
   - 실시간 진행 상황 모니터링 가능

2. **Coarse-grained 업로드**
   - 더 큰 세그먼트 단위로 업로드
   - 네트워크 요청 수 감소
   - 중간 크기의 메모리 사용

3. **Batch 업로드**
   - 모든 세그먼트를 메모리에 모았다가 한 번에 업로드
   - 네트워크 요청 최소화
   - 높은 메모리 사용량

4. **Stream 업로드** (동시성 지원)
   - goroutine을 사용한 병렬 업로드
   - worker 수 동적 설정 가능 (기본값: 4)
   - 효율적인 리소스 사용과 빠른 업로드 속도

### 2. CSV 파일 조회
- S3에 저장된 세그먼트 파일 조회
- offset과 limit을 통한 페이지네이션 지원
- 세그먼트 단위로 분할된 파일 자동 처리
- 필요한 세그먼트만 선택적으로 다운로드

## API 엔드포인트

### 업로드 엔드포인트

1. 기본 업로드 (fine-grained):
```
POST /cht/v1/secure-file/csv/{channelId}/{fileName}
```

2. 테스트용 엔드포인트:
```
# Fine-grained upload (1,000 rows/segment)
POST /test/fine-grained/csv/{channelId}/{fileName}

# Coarse-grained upload (10,000 rows/segment)
POST /test/coarse-grained/csv/{channelId}/{fileName}

# Batch upload (1,000 rows/segment, uploaded in batch)
POST /test/batch-upload/csv/{channelId}/{fileName}

# Stream upload with configurable workers
POST /test/stream-upload/csv/{channelId}/{fileName}?workers={numWorkers}
```

- `channelId`: 채널 식별자
- `fileName`: 업로드할 CSV 파일명
- `workers`: (stream 모드) 동시 업로드 worker 수 (기본값: 4)

### 조회 엔드포인트
```
GET /admin/cht/v1/secure-file/csv-upload/csv/{channelId}/{timestamp}?offset={offset}&limit={limit}
```

- `channelId`: 채널 식별자
- `timestamp`: 업로드 시간 (형식: YYYY-MM-DD-HH-mm-ss)
- `offset`: 건너뛸 라인 수 (기본값: 0)
- `limit`: 반환할 라인 수 (기본값: 100, 최대: 1000)

## 성능 측정

각 요청에 대해 다음 정보가 로깅됩니다:
- 요청 시작 시간
- 요청 완료 시간
- 총 소요 시간
- 세그먼트별 처리 통계 (업로드 모드)
  - 처리된 행 수
  - 데이터 크기
  - 업로드 속도 (MB/s)

## 설정

### AWS 설정
- 리전: ap-northeast-2
- 프로파일: ch-dev
- 버킷: bin.exp.channel.io

## 실행 방법

```bash
go run .
```

서버는 8080 포트에서 실행됩니다.

## 사용 예시

```bash
# Fine-grained upload
curl -X POST -T "data.csv" "http://localhost:8080/cht/v1/secure-file/csv/1/data.csv"

# Stream upload with 8 workers
curl -X POST -T "data.csv" "http://localhost:8080/test/stream-upload/csv/1/data.csv?workers=8"

# Query uploaded file
curl "http://localhost:8080/admin/cht/v1/secure-file/csv-upload/csv/1/2024-03-19-10-45-09?offset=0&limit=100"
```

## Performance Tests

### Source Codes

https://github.com/isuh88/csv-query-test

### Test Environments

- OS: darwin 24.0.0
- Go 버전: 1.21.0
- S3 Bucket: bin-secure.exp.channel.io

### Test Data

- 500K rows, 83.0MB
    
    https://s3.ap-northeast-2.amazonaws.com/bin-secure.exp.channel.io/customers-500000.csv
    
    [customers-500000.zip](attachment:dde530a7-9a67-4c96-9e39-2d342e1f4200:customers-500000.zip)
    
- 1M rows, 166.1MB
    
    https://s3.ap-northeast-2.amazonaws.com/bin-secure.exp.channel.io/customers-1000000.csv
    
    [customers-1000000.zip](attachment:382b18b4-8461-4f92-94c7-40465ec5d29c:customers-1000000.zip)
    
- 2M rows, 333.2MB
    
    https://s3.ap-northeast-2.amazonaws.com/bin-secure.exp.channel.io/customers-2000000.csv
    
    [customers-2000000.zip](attachment:43bf2b0f-199f-4d7b-8bd0-5eaa80e3c28b:customers-2000000.zip)
    

### Test Scenarios

#### 1. 파일 시작 부분 조회 (offset=0)

각 구현 방식별로 파일의 시작 부분을 조회합니다.

1. customers-500000.csv

| 구현 방식 | offset | limit | 응답 시간 (ms) |
| --- | --- | --- | --- |
| Original | 0 | 100 | 103.87 |
| Cached | 0 | 100 | 3865.37 |
| Fast Skip | 0 | 100 | 98.28 |
| Optimized | 0 | 100 | 3928.37 |

b. customers-1000000.csv

| 구현 방식 | offset | limit | 응답 시간 (ms) |
| --- | --- | --- | --- |
| Original | 0 | 100 | 126.77 |
| Cached | 0 | 100 | 7270.68 |
| Fast Skip | 0 | 100 | 89.56 |
| Optimized | 0 | 100 | 8870.14 |

c. customers-2000000.csv

| 구현 방식 | offset | limit | 응답 시간 (ms) |
| --- | --- | --- | --- |
| Original | 0 | 100 | 84.73 |
| Cached | 0 | 100 | 18520.75 |
| Fast Skip | 0 | 100 | 94.83 |
| Optimized | 0 | 100 | 20705.48 |

#### 2. 파일 마지막 부분 조회 (offset=last)

각 구현 방식별로 파일의 마지막 부분을 조회합니다.

a. customers-500000.csv, offset=499900

| 구현 방식 | offset | limit | 응답 시간 (ms) |
| --- | --- | --- | --- |
| Original | 499900 | 100 | 4045.11 |
| Cached | 499900 | 100 | 4968.36 |
| Fast Skip | 499900 | 100 | 4515.97 |
| Optimized | 499900 | 100 | 4453.03 |

b. customers-1000000.csv, offset=999900

| 구현 방식 | offset | limit | 응답 시간 (ms) |
| --- | --- | --- | --- |
| Original | 999900 | 100 | 8755.99 |
| Cached | 999900 | 100 | 6812.81 |
| Fast Skip | 999900 | 100 | 9574.21 |
| Optimized | 999900 | 100 | 8622.57 |

c. customers-2000000.csv, offset=1999900

| 구현 방식 | offset | limit | 응답 시간 (ms) |
| --- | --- | --- | --- |
| Original | 1999900 | 100 | 17034.32 |
| Cached | 1999900 | 100 | 19765.02 |
| Fast Skip | 1999900 | 100 | 18249.76 |
| Optimized | 1999900 | 100 | 18614.09 |

#### 3. 캐시된 상태에서 마지막 부분 조회

1회 파일 요청 이후 마지막 부분을 다시 조회합니다.

⚠️ `Original` , `Fast Skip` 의 경우 2번 결과를 그대로 기입했습니다.

1. customers-500000.csv, offset=499900

| 구현 방식 | offset | limit | 첫 요청 (ms) | 두 번째 요청 (ms) |
| --- | --- | --- | --- | --- |
| Original | 499900 | 100 | 4045.11 | 4045.11 |
| Cached | 499900 | 100 | 4313.80 | 42.95 |
| Fast Skip | 499900 | 100 | 4515.97 | 4515.97 |
| Optimized | 499900 | 100 | 4951.08 | 50.79 |

b. customers-1000000.csv, offset=999900

| 구현 방식 | offset | limit | 첫 요청 (ms) | 두 번째 요청 (ms) |
| --- | --- | --- | --- | --- |
| Original | 999900 | 100 | 8755.99 | 8755.99 |
| Cached | 999900 | 100 | 4856.39 | 133.51 |
| Fast Skip | 999900 | 100 | 9574.21 | 9574.21 |
| Optimized | 999900 | 100 | 5504.54 | 70.88 |

c. customers-2000000.csv, offset=1999900

| 구현 방식 | offset | limit | 첫 요청 (ms) | 두 번째 요청 (ms) |
| --- | --- | --- | --- | --- |
| Original | 1999900 | 100 | 17034.32 | 17034.32 |
| Cached | 1999900 | 100 | 12724.96 | 319.30 |
| Fast Skip | 1999900 | 100 | 18249.76 | 18249.76 |
| Optimized | 1999900 | 100 | 14670.20 | 221.92 |
