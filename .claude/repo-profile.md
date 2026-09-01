# repo 프로필

- 대상: `ai-agent-proto`
- 관례 출처: `mc-observability` 를 따른다 (사용자 승인, 2026-09-02)

## 커밋 스타일

| 항목 | 값 |
|------|-----|
| 언어 | 영어 |
| 접두사 | conventional (`fix:` / `feat:` / `docs:` / `build:` / `refactor:` / `perf:` / `chore:`) |
| 스코프 | 선택. 붙일 때는 `fix(deploy):` 처럼 패키지 이름 |
| 제목 길이 | 중앙값 80자 수준. 짧게 자르지 말고 무엇을 했는지 다 쓴다 |
| 본문 | 드물게만 (mc-observability 기준 3%). 제목으로 안 되는 배경이 있을 때만 |
| Signed-off-by | 안 씀 |
| Co-Authored-By | 안 씀 |
| 티켓번호 | 제목에 안 넣음 |

## 코드 스타일

| 항목 | 값 |
|------|-----|
| 언어 | Go |
| 인덴트 | gofmt (탭) |
| 주석 언어 | 영어 |
| 문서(md) 언어 | 한글 |

## 훅

`~/.claude/skills/hooks/install-hooks.sh` 로 설치됨 (프로필 `private`).
