# CSV Query Service

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

## 성능 비교 방법

1. 다양한 크기의 파일에 대해 테스트
2. 다양한 offset 값으로 테스트
3. 캐시 효과를 보기 위해 동일 파일 반복 요청
4. 각 구현의 소요 시간 비교
