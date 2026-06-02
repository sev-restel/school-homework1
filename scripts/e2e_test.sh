#!/usr/bin/env bash
# End-to-end проверка API School Homework.
# Требует запущенный сервер на :8080 и поднятый Postgres.
set -u
BASE="http://localhost:8080"
PY="${PY:-python}"
PASS=0; FAIL=0
TMP=$(mktemp -d)
echo "Файл-заглушка для загрузок" > "$TMP/hw.txt"
echo "Ответ ученика" > "$TMP/answer.txt"

# jget <json> <python-path>  — вытащить поле из JSON
jget() { printf '%s' "$1" | "$PY" -c "import sys,json;d=json.load(sys.stdin);print($2)" 2>/dev/null; }

# req METHOD PATH [curl-args...] -> печатает 'CODE\nBODY'
req() {
  local m=$1 p=$2; shift 2
  curl -s -X "$m" "$BASE$p" -w $'\n%{http_code}' "$@"
}

check() { # check "name" expected_code actual_code
  if [ "$2" = "$3" ]; then echo "  ✓ $1 ($3)"; PASS=$((PASS+1));
  else echo "  ✗ $1 — ожидал $2, получил $3"; FAIL=$((FAIL+1)); fi
}

split() { CODE=$(printf '%s' "$1" | tail -n1); BODY=$(printf '%s' "$1" | sed '$d'); }

# ВАЖНО: данные только ASCII — MSYS-bash на Windows перекодирует не-ASCII argv
# в cp1251, и до сервера дойдёт битый UTF-8. Реальный фронт шлёт UTF-8 через fetch.
echo "=== 1. Регистрация ==="
R=$(req POST /api/register -F full_name="Admin Glavny" -F password="admin123" -F invite_code="SECRET_ADMIN_456"); split "$R"
check "register admin" 200 "$CODE"; ADMIN_U=$(jget "$BODY" "d['username']")
R=$(req POST /api/register -F full_name="Teacher One" -F password="teach123" -F subject="Math" -F invite_code="SECRET_TEACHER_123"); split "$R"
check "register teacher1" 200 "$CODE"; T1_U=$(jget "$BODY" "d['username']")
R=$(req POST /api/register -F full_name="Teacher Two" -F password="teach123" -F subject="History" -F invite_code="SECRET_TEACHER_123"); split "$R"
check "register teacher2" 200 "$CODE"; T2_U=$(jget "$BODY" "d['username']")
R=$(req POST /api/register -F full_name="Student Vasilev" -F password="stud123"); split "$R"
check "register student" 200 "$CODE"; S_U=$(jget "$BODY" "d['username']")
R=$(req POST /api/register -F full_name="Some One" -F password="123"); split "$R"
check "register: короткий пароль отклонён" 400 "$CODE"

echo "=== 2. Логин ==="
R=$(req POST /api/login -F username="$ADMIN_U" -F password="admin123"); split "$R"; check "login admin" 200 "$CODE"; ADMIN_TOK=$(jget "$BODY" "d['token']")
R=$(req POST /api/login -F username="$T1_U" -F password="teach123"); split "$R"; check "login teacher1" 200 "$CODE"; T1_TOK=$(jget "$BODY" "d['token']")
R=$(req POST /api/login -F username="$T2_U" -F password="teach123"); split "$R"; check "login teacher2" 200 "$CODE"; T2_TOK=$(jget "$BODY" "d['token']")
R=$(req POST /api/login -F username="$S_U" -F password="stud123"); split "$R"; check "login student" 200 "$CODE"; S_TOK=$(jget "$BODY" "d['token']")
R=$(req POST /api/login -F username="$S_U" -F password="wrong"); split "$R"; check "login: неверный пароль" 401 "$CODE"

AH() { echo "-HAuthorization: Bearer $1"; }

echo "=== 3. Учитель создаёт ДЗ ==="
R=$(req POST /api/homeworks -H "Authorization: Bearer $T1_TOK" -F file=@"$TMP/hw.txt" -F subject="Math" -F description="par5"); split "$R"
check "teacher1 upload homework" 200 "$CODE"
R=$(req POST /api/homeworks -H "Authorization: Bearer $T2_TOK" -F file=@"$TMP/hw.txt" -F subject="History" -F description="report"); split "$R"
check "teacher2 upload homework" 200 "$CODE"
R=$(req POST /api/homeworks -H "Authorization: Bearer $S_TOK" -F file=@"$TMP/hw.txt"); split "$R"
check "student upload homework запрещён" 403 "$CODE"

echo "=== 4. Видимость ДЗ по ролям ==="
R=$(req GET /api/homeworks -H "Authorization: Bearer $T1_TOK"); split "$R"
N=$(jget "$BODY" "len(d)"); check "teacher1 видит только свои (1 шт)" 1 "$N"
HW_ID=$(jget "$BODY" "d[0]['id']")
R=$(req GET /api/homeworks -H "Authorization: Bearer $S_TOK"); split "$R"
N=$(jget "$BODY" "len(d)"); check "студент БЕЗ класса не видит ДЗ (0 шт)" 0 "$N"

echo "=== 5. Заявка в класс ==="
R=$(req POST /api/requests -H "Authorization: Bearer $S_TOK" -H "Content-Type: application/json" -d '{"class_name":"5A"}'); split "$R"
check "student создаёт заявку" 201 "$CODE"
R=$(req GET "/api/requests?status=pending" -H "Authorization: Bearer $T1_TOK"); split "$R"
check "teacher видит заявки" 200 "$CODE"; REQ_ID=$(jget "$BODY" "d[0]['id']")
R=$(req PATCH /api/requests/$REQ_ID -H "Authorization: Bearer $T1_TOK" -H "Content-Type: application/json" -d '{"status":"accepted"}'); split "$R"
check "teacher принимает заявку" 200 "$CODE"
R=$(req GET /api/profile -H "Authorization: Bearer $S_TOK"); split "$R"
CLS=$(jget "$BODY" "d['class']"); check "у студента появился класс 5A" "5A" "$CLS"
R=$(req GET /api/homeworks -H "Authorization: Bearer $S_TOK"); split "$R"
N=$(jget "$BODY" "len(d)"); check "студент С классом видит ДЗ (2 шт)" 2 "$N"

echo "=== 6. Сдача и оценка ==="
R=$(req POST /api/submissions -H "Authorization: Bearer $S_TOK" -F homework_id="$HW_ID" -F file=@"$TMP/answer.txt"); split "$R"
check "student сдаёт работу" 201 "$CODE"
R=$(req GET /api/submissions/$HW_ID -H "Authorization: Bearer $T1_TOK"); split "$R"
check "teacher видит работы" 200 "$CODE"; SUB_ID=$(jget "$BODY" "d[0]['id']"); SUB_FP=$(jget "$BODY" "d[0]['filepath']")
R=$(req GET /api/submissions/$HW_ID -H "Authorization: Bearer $T2_TOK"); split "$R"
check "чужой учитель НЕ видит работы по чужому ДЗ" 403 "$CODE"
R=$(req PATCH /api/submissions/$SUB_ID/grade -H "Authorization: Bearer $T2_TOK" -H "Content-Type: application/json" -d '{"grade":4}'); split "$R"
check "чужой учитель НЕ может оценить чужую работу" 404 "$CODE"
R=$(req PATCH /api/submissions/$SUB_ID/grade -H "Authorization: Bearer $T1_TOK" -H "Content-Type: application/json" -d '{"grade":5}'); split "$R"
check "teacher ставит оценку 5" 200 "$CODE"
R=$(req PATCH /api/submissions/$SUB_ID/grade -H "Authorization: Bearer $T1_TOK" -H "Content-Type: application/json" -d '{"grade":9}'); split "$R"
check "оценка вне 1..5 отклонена" 400 "$CODE"
R=$(req GET /api/grades -H "Authorization: Bearer $S_TOK"); split "$R"
G=$(jget "$BODY" "d[0]['grade']"); check "student видит оценку 5" 5 "$G"

echo "=== 7. Скачивание файла ==="
R=$(req GET "/$SUB_FP"); split "$R"; check "скачивание загруженного файла" 200 "$CODE"

echo "=== 8. Расписание ==="
R=$(req POST /api/schedule -H "Authorization: Bearer $ADMIN_TOK" -H "Content-Type: application/json" -d "{\"class_name\":\"5A\",\"week_parity\":\"odd\",\"day_of_week\":1,\"lesson_num\":1,\"subject\":\"Math\",\"teacher_id\":1,\"room\":\"201\"}"); split "$R"
check "admin создаёт расписание" 201 "$CODE"; SCH_ID=$(jget "$BODY" "d['id']")
R=$(req GET "/api/schedule?class=5A&week=odd" -H "Authorization: Bearer $ADMIN_TOK"); split "$R"
check "GET расписание" 200 "$CODE"
R=$(req PATCH /api/schedule/$SCH_ID -H "Authorization: Bearer $S_TOK" -H "Content-Type: application/json" -d '{"room":"999"}'); split "$R"
check "БАГ-ФИКС: студент НЕ может править расписание" 403 "$CODE"
R=$(req DELETE /api/schedule/$SCH_ID -H "Authorization: Bearer $ADMIN_TOK"); split "$R"
check "admin удаляет расписание" 200 "$CODE"

echo "=== 9. Logout ==="
R=$(req POST /api/logout -H "Authorization: Bearer $S_TOK"); split "$R"; check "logout" 200 "$CODE"
R=$(req GET /api/homeworks -H "Authorization: Bearer $S_TOK"); split "$R"; check "после logout токен недействителен" 401 "$CODE"

rm -rf "$TMP"
echo ""
echo "ИТОГО: PASS=$PASS  FAIL=$FAIL"
[ "$FAIL" = "0" ]
