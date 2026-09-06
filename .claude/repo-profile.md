# repo 프로필

<!-- generated-by: repo-profile.sh - 손으로 고치면 --save 가 덮어쓰지 않습니다 -->

- 대상: `ai-agent-proto`
- 소유: **내 repo** (내 커밋 12 개, `임수현`)
- 분석 범위: repo 전체 (내 커밋은 12 개뿐이라 전체를 본다) 중 최근 12 개 (Revert/Merge 제외)
- 뽑은 날: 2026-09-06

## 커밋 스타일

| 항목 | 값 |
|------|-----|
| 언어 | **영어** (한글 제목 0/12 = 0%) |
| 접두사 | **conventional (fix: / feat: ...)** (conventional 12 · 스코프형 0 · 없음 0) |
| 다중 스코프 | `모듈1, 모듈2:` 형태 0 개 |
| 제목 길이 | 중앙값 **74자** · p90 81자 · 최대 90자 |
| 본문 | 11/12 = 91% 가 본문 있음 |
| Signed-off-by | 0 개 (안 씀) |
| 머지 커밋 | 0 개 (전부 rebase/cherry-pick) |
| 티켓번호 | 제목에 0 개 (제목에 안 넣음) |

### 접두사 top 10
```
      5 feat:
      5 docs:
      1 fix:
      1 chore:
```

### 접두사 뒤 첫 단어 top 8
```
      2 record
      2 add
      1 show
      1 read
      1 re-audit
      1 rank
      1 open
      1 match
```

### 최근 제목 10개
```
docs: record what the matcher and the accelerator probe changed and what they do not cover
feat: read the accelerator on deployed nodes and compare it to the catalog
feat: rank catalog entries against the operator's own words
docs: re-audit leftover resources across every connection and every local session
docs: audit the cloud resources the live verification created and removed
docs: show with a control experiment that the serving port rule is what opens it
feat: let a deployment pin the security group template it starts from
chore: add the Apache 2.0 license the project publishes under
docs: record the end-to-end serving port verification and what it does not prove
feat: open the AI application serving port on the deployment security group
```

## 코드 스타일

| 항목 | 값 |
|------|-----|
| 주력 확장자 | go (41개) |
| 표본 | 40 개 파일, 6827 줄 |
| 인덴트 | **탭** (탭 3128 줄 / 스페이스 1745 줄) |
| 행 길이 | 80칸 초과 5% · 100칸 2% · 120칸 0% |
| 주석 언어 | **영어** (주석 1070 줄 중 한글 2 줄) |
| 주석 형태 | `//` 901 · `/* */` 1 |

## 확인받을 것

> 이 repo 는 영어 + conventional (fix: / feat: ...) 이고 제목 중앙값이 74자입니다.
> 주석은 영어, 인덴트는 탭 입니다. 이대로 갈까요?

## 손으로 덧붙인 결정

**이 절은 `repo-profile.sh` 가 만들지 않는다.** 스크립트가 로그에서 못 뽑는 결정을 여기 적는다.
`--save` 로 프로필을 다시 뽑으면 이 절이 사라지므로, 그때 `.bak` 에서 다시 옮겨 붙인다.

| 항목 | 값 |
|------|-----|
| 관례 출처 | `mc-observability` 를 따른다 (사용자 승인, 2026-09-02) |
| 스코프 | 선택. 붙일 때는 `fix(deploy):` 처럼 패키지 이름 |
| 문서(md) 언어 | 한글. 코드 주석은 영어다 |
| 훅 프로필 | `private` |

### 본문 비율이 어긋난다

이전 프로필에는 본문을 **드물게만** 쓴다고 적혀 있었다(`mc-observability` 기준 3%).
그런데 이 repo 의 실제 로그는 **12개 중 11개(91%)가 본문을 달았다.**

어느 쪽을 따를지 정해지지 않았다. 지금은 실측값을 위 표에 그대로 두었다.
