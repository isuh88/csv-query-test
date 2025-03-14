# CSV Query Service

## 목차
- [프로젝트 구조](#프로젝트-구조)
- [주요 기능](#주요-기능)
  - [CSV 파일 조회](#1-csv-파일-조회)
  - [최적화 전략](#2-최적화-전략)
- [API 엔드포인트](#api-엔드포인트)
  - [사용 가능한 파일 목록](#사용-가능한-파일-목록)
- [성능 측정](#성능-측정)
- [설정](#설정)
  - [AWS 설정](#aws-설정)
- [실행 방법](#실행-방법)
- [사용 예시](#사용-예시)
- [성능 테스트](#performance-tests)
  - [테스트 환경](#test-environments)
  - [테스트 데이터](#test-data)
  - [테스트 시나리오](#test-scenarios)

이 프로젝트는 S3에 저장된 대용량 CSV 파일을 효율적으로 조회하기 위한 HTTP 서비스입니다. 파일의 특정 부분만을 조회할 수 있도록 페이지네이션을 지원하며, 다양한 최적화 전략을 비교할 수 있도록 구현되어 있습니다.

## 프로젝트 구조

```
.
├── main.go              # 서버 진입점 및 라우팅 설정
├── handler.go           # 기본 핸들러 구현
├── s3_client.go         # 기본 S3 클라이언트
├── cached_s3_client.go  # 캐싱이 적용된 S3 클라이언트
├── optimized_handler.go # 최적화된 핸들러들의 구현
└── time_check.go       # 성능 측정 유틸리티
```

## 주요 기능

### 1. CSV 파일 조회
- S3 버킷에서 CSV 파일을 조회
- offset과 limit을 통한 페이지네이션 지원
- 파일명을 query parameter로 받아 동적 조회 가능

### 2. 최적화 전략

프로젝트는 4가지 다른 구현을 제공하여 성능을 비교할 수 있습니다:

1. **기본 구현** (`/csv/original`)
   - 단순한 라인 단위 읽기
   - 최적화 없음

2. **캐시 적용** (`/csv/cached`)
   - S3에서 다운로드한 파일을 메모리에 캐싱
   - 반복된 요청 시 성능 향상

3. **빠른 스킵** (`/csv/fast-skip`)
   - 버퍼를 사용한 최적화된 라인 스킵
   - 큰 offset 값에서 성능 향상

4. **완전 최적화** (`/csv/optimized`)
   - 캐싱과 빠른 스킵 모두 적용
   - 최상의 성능 제공

## API 엔드포인트

모든 엔드포인트는 동일한 query parameter를 지원합니다:

```
GET /csv/{endpoint}?file={filename}&offset={offset}&limit={limit}
```

- `endpoint`: original, cached, fast-skip, optimized 중 하나
- `file`: CSV 파일명 (예: customers-500000.csv)
- `offset`: 건너뛸 라인 수 (기본값: 0)
- `limit`: 반환할 라인 수 (기본값: 100)

### 사용 가능한 파일 목록
- customers-500000.csv
- customers-1000000.csv
- customers-2000000.csv

## 성능 측정

각 요청에 대해 다음 정보가 로깅됩니다:
- 요청 시작 시간
- 요청 완료 시간
- 총 소요 시간

## 설정

### AWS 설정
- 리전: ap-northeast-2
- 프로파일: ch-dev
- 버킷: bin-secure.exp.channel.io

## 실행 방법

```bash
go run .
```

서버는 8080 포트에서 실행됩니다.

## 사용 예시

```bash
# 기본 구현
curl "http://localhost:8080/csv/original?file=customers-500000.csv&offset=1000&limit=10"

# 캐시 사용
curl "http://localhost:8080/csv/cached?file=customers-500000.csv&offset=1000&limit=10"

# 빠른 스킵
curl "http://localhost:8080/csv/fast-skip?file=customers-500000.csv&offset=1000&limit=10"

# 모든 최적화 적용
curl "http://localhost:8080/csv/optimized?file=customers-500000.csv&offset=1000&limit=10"
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
