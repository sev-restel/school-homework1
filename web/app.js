// =========================================================
// Вспомогательные данные
// =========================================================
const DAYS = ["", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"];

// Класс текущего ученика (для расписания и подписей). Заполняется в loadStudentData.
let studentClass = "";
// Текущее выбранное задание на вкладке "Работы учеников" (для авто-обновления).
let subsCurrentHw = null;
// Таймер авто-обновления данных (чтобы не жать F5).
let pollTimer = null;

// =========================================================
// Токен / авторизация
// =========================================================
function getToken()  { return localStorage.getItem("token"); }
function setAuth(d)  {
  localStorage.setItem("token",    d.token);
  localStorage.setItem("role",     d.role);
  localStorage.setItem("username", d.username);
}
function clearAuth() {
  ["token", "role", "username"].forEach(k => localStorage.removeItem(k));
}

// =========================================================
// Базовый HTTP-запрос
// Все запросы идут на тот же origin, что и фронт.
// Bearer-токен подставляется автоматически.
// =========================================================
async function request(path, { method = "GET", body = null, json = false } = {}) {
  const headers = {};
  const token = getToken();
  if (token) headers["Authorization"] = "Bearer " + token;
  if (json)  headers["Content-Type"]  = "application/json";

  const res  = await fetch(path, { method, headers, body });
  const text = await res.text();
  if (!res.ok) throw new Error(text.trim() || "HTTP " + res.status);
  try { return JSON.parse(text); } catch { return text; }
}

// =========================================================
// API-слой — все эндпоинты в одном месте
// =========================================================
const api = {
  // Auth
  register: (fd)    => request("/api/register", { method: "POST", body: fd }),
  login:    (fd)    => request("/api/login",    { method: "POST", body: fd }),
  logout:   ()      => request("/api/logout",   { method: "POST" }),
  profile:  ()      => request("/api/profile"),

  // Homework (RESTful: один путь /api/homeworks, разные методы)
  listHomeworks:   (subject)  => request("/api/homeworks" + (subject ? "?subject=" + encodeURIComponent(subject) : "")),
  uploadHomework:  (fd)       => request("/api/homeworks", { method: "POST", body: fd }),
  deleteHomework:  (id)       => request("/api/homeworks/" + id, { method: "DELETE" }),
  updateHomework:  (id, data) => request("/api/homeworks/" + id, { method: "PATCH", body: JSON.stringify(data), json: true }),
  replaceHomework: (id, fd)   => request("/api/homeworks/" + id, { method: "PUT", body: fd }),

  // Schedule
  getSchedule:    (cls, week) => request(`/api/schedule?class=${encodeURIComponent(cls)}&week=${week}`),
  createSchedule: (data)      => request("/api/schedule", { method: "POST", body: JSON.stringify(data), json: true }),
  updateSchedule: (id, data)  => request("/api/schedule/" + id, { method: "PATCH", body: JSON.stringify(data), json: true }),
  deleteSchedule: (id)        => request("/api/schedule/" + id, { method: "DELETE" }),

  // Submissions (ученик сдаёт, учитель проверяет)
  createSubmission: (fd)         => request("/api/submissions", { method: "POST", body: fd }),
  getSubmissions:   (hwId)       => request("/api/submissions/" + hwId),
  gradeSubmission:  (id, grade)  => request(`/api/submissions/${id}/grade`, { method: "PATCH", body: JSON.stringify({ grade }), json: true }),

  // Grades
  getGrades: () => request("/api/grades"),

  // Requests (заявки в класс)
  getRequests:   (status) => request("/api/requests?status=" + (status || "pending")),
  createRequest: (cls)    => request("/api/requests", { method: "POST", body: JSON.stringify({ class_name: cls }), json: true }),
  decideRequest: (id, st) => request("/api/requests/" + id, { method: "PATCH", body: JSON.stringify({ status: st }), json: true }),

  // Teachers list (для формы расписания)
  listTeachers: () => request("/api/teachers"),

  // Расписание самого учителя (его уроки во всех классах)
  getMySchedule: (week) => request("/api/schedule?mine=1&week=" + week),

  // Управление пользователями (admin)
  listUsers:   ()             => request("/api/users"),
  kickUser:    (id)           => request("/api/users/" + id + "/kick", { method: "PATCH" }),
  setPassword: (id, password) => request("/api/users/" + id + "/password", { method: "PATCH", body: JSON.stringify({ password }), json: true }),
  deleteUser:  (id)           => request("/api/users/" + id, { method: "DELETE" }),
};

// =========================================================
// UI-утилиты
// =========================================================
const $ = sel => document.querySelector(sel);

// Экранирование HTML, чтобы данные из БД не стали кодом
const h = s => String(s).replace(/[&<>"']/g, c =>
  ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])
);

function toast(msg, type = "success") {
  const el = $("#toast");
  el.textContent = msg;
  el.className   = "toast " + type;
  el.hidden      = false;
  clearTimeout(toast._t);
  toast._t = setTimeout(() => { el.hidden = true; }, 3500);
}

// Показывает нужную секцию по роли, остальные прячет
function showSection(role) {
  $("#auth-screen").hidden     = role !== null;
  $("#app-shell").hidden       = role === null;
  $("#student-section").hidden = role !== "student";
  $("#teacher-section").hidden = role !== "teacher";
  $("#admin-section").hidden   = role !== "admin";
}

// Иконка-скачивание для ссылок
const downloadIcon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="M7 10l5 5 5-5"/><path d="M12 15V3"/></svg>`;

// Ссылка на скачивание файла по сохранённому пути ("uploads/xxx")
function downloadLink(filepath, label) {
  const url  = "/" + String(filepath).replace(/^\/+/, "");
  const name = label || filepath.split("/").pop();
  return `<a class="download-link" href="${h(url)}" download>${downloadIcon}${h(name)}</a>`;
}

// =========================================================
// Переключение вкладок auth (Вход / Регистрация)
// =========================================================
function activateAuthTab(name) {
  document.querySelectorAll(".tab").forEach(b => b.classList.toggle("active", b.dataset.tab === name));
  document.querySelectorAll(".tab-content").forEach(c => c.classList.remove("active"));
  $(name === "login" ? "#login-form" : "#register-form").classList.add("active");
}

document.querySelectorAll(".tab").forEach(btn => {
  btn.addEventListener("click", () => activateAuthTab(btn.dataset.tab));
});

// Ссылки "Нет аккаунта? Зарегистрироваться" / "Уже есть аккаунт? Войти"
document.querySelectorAll(".link[data-goto]").forEach(btn => {
  btn.addEventListener("click", () => activateAuthTab(btn.dataset.goto));
});

// =========================================================
// Переключение панелей внутри роли (role-tabs)
// =========================================================
function setupRoleTabs(sectionId) {
  const sec = document.getElementById(sectionId);
  if (!sec) return;
  sec.querySelectorAll(".role-tab").forEach(btn => {
    btn.addEventListener("click", () => {
      sec.querySelectorAll(".role-tab").forEach(b => b.classList.remove("active"));
      sec.querySelectorAll(".panel").forEach(p => p.classList.remove("active"));
      btn.classList.add("active");
      const panel = document.getElementById(btn.dataset.panel);
      if (panel) panel.classList.add("active");
    });
  });
}

// Программно переключить панель без клика по кнопке
function switchPanel(sectionId, panelId) {
  const sec = document.getElementById(sectionId);
  if (!sec) return;
  sec.querySelectorAll(".role-tab").forEach(b =>
    b.classList.toggle("active", b.dataset.panel === panelId)
  );
  sec.querySelectorAll(".panel").forEach(p =>
    p.classList.toggle("active", p.id === panelId)
  );
}

// =========================================================
// ===  AUTH  =============================================
// =========================================================
$("#login-form").addEventListener("submit", async e => {
  e.preventDefault();
  try {
    const data = await api.login(new FormData(e.target));
    setAuth(data);
    await bootstrap();
    toast("Добро пожаловать, " + data.username + "!");
  } catch (err) { toast(err.message, "error"); }
});

$("#register-form").addEventListener("submit", async e => {
  e.preventDefault();
  try {
    const data = await api.register(new FormData(e.target));
    toast("Зарегистрирован. Ваш username: " + data.username);
    $("#login-form [name=username]").value = data.username;
    document.querySelectorAll(".tab")[0].click();
  } catch (err) { toast(err.message, "error"); }
});

$("#logout-btn").addEventListener("click", async () => {
  try { await api.logout(); } catch (_) {}
  stopPolling();
  clearAuth();
  showSection(null);
  toast("Вы вышли из системы");
});

// =========================================================
// ===  STUDENT  ==========================================
// =========================================================

async function loadStudentData(prof) {
  studentClass = prof.class || "";
  showStudentClass(prof);
  $("#stu-sched-class").textContent = studentClass
    ? "Класс " + studentClass
    : "Вы пока не записаны в класс";
  // Грузим задания, оценки, учителей (для имён в расписании) и расписание
  await Promise.all([loadStuHomeworks(), loadStuGrades(), loadTeachers()]);
  await loadStuSchedule();
}

// Рендер расписания сеткой «уроки × дни» (только просмотр).
// mode: 'student' → во второй строке учитель+кабинет; 'teacher' → класс+кабинет.
function readonlyScheduleGridHTML(map, mode) {
  let html = `<div class="table-wrap"><table class="sched-grid"><thead><tr><th>Урок</th>`;
  for (let d = 1; d <= 6; d++) html += `<th>${DAYS_SHORT[d]}</th>`;
  html += `</tr></thead><tbody>`;
  for (let l = 1; l <= SCHED_LESSONS; l++) {
    html += `<tr><th class="sched-grid__num">${l}</th>`;
    for (let d = 1; d <= 6; d++) {
      const les = map[`${d}_${l}`];
      if (les) {
        const room = les.room ? " · каб. " + h(les.room) : "";
        const line2 = mode === "teacher"
          ? "класс " + h(les.class_name) + room
          : h(teacherNameById(les.teacher_id)) + room;
        html += `<td class="sched-cell sched-cell--filled sched-cell--ro">
          <div class="sched-cell__subj">${h(les.subject)}</div>
          <div class="sched-cell__meta">${line2}</div></td>`;
      } else {
        html += `<td class="sched-cell sched-cell--empty sched-cell--ro"></td>`;
      }
    }
    html += `</tr>`;
  }
  html += `</tbody></table></div>`;
  return html;
}

// Расписание ученика (по его классу)
async function loadStuSchedule() {
  const wrap = $("#stu-sched-grid");
  if (!studentClass) {
    wrap.innerHTML = `<p class="hint">Вступите в класс, чтобы увидеть расписание.</p>`;
    return;
  }
  const week = $("#stu-sched-week").value;
  try {
    const list = (await api.getSchedule(studentClass, week)) || [];
    const map = {};
    for (const s of list) map[`${s.day_of_week}_${s.lesson_num}`] = s;
    wrap.innerHTML = readonlyScheduleGridHTML(map, "student");
  } catch (err) { toast("Ошибка загрузки расписания: " + err.message, "error"); }
}

// Список заданий от учителей + заполнение селекта "Сдать работу"
async function loadStuHomeworks() {
  try {
    const items = (await api.listHomeworks()) || [];

    const ul = $("#stu-hw-list");
    ul.innerHTML = items.length === 0 ? '<li class="empty">Заданий пока нет.</li>' : "";

    const sel = $("#submit-hw-select");
    const prev = sel.value;
    sel.innerHTML = '<option value="">— выберите задание —</option>';

    for (const hw of items) {
      const label = [hw.subject, hw.filename].filter(Boolean).join(" — ");

      // Карточка задания
      const li = document.createElement("li");
      li.innerHTML = `
        <div class="item-title">${h(hw.filename)}</div>
        <div class="item-meta">${[
          hw.class_name ? "Класс: "   + h(hw.class_name) : null,
          hw.subject    ? "Предмет: " + h(hw.subject)    : null,
        ].filter(Boolean).join(" · ") || "—"}</div>
        ${hw.description ? `<div class="item-desc"><span class="item-desc__label">Задание:</span> ${h(hw.description)}</div>` : ""}
        <div class="item-actions">
          ${hw.filepath ? downloadLink(hw.filepath, "Скачать файл") : ""}
          <button class="btn-outline" data-hw-id="${hw.id}">Сдать работу</button>
        </div>`;
      li.querySelector("[data-hw-id]").addEventListener("click", () => {
        $("#submit-hw-select").value = hw.id;
        switchPanel("student-section", "stu-submit");
      });
      ul.appendChild(li);

      // Опция в селект
      const opt = document.createElement("option");
      opt.value       = hw.id;
      opt.textContent = label;
      sel.appendChild(opt);
    }

    sel.value = prev; // восстанавливаем выбор
  } catch (err) { toast("Ошибка загрузки заданий: " + err.message, "error"); }
}

// Оценки ученика
async function loadStuGrades() {
  try {
    const grades = (await api.getGrades()) || [];
    const ul = $("#stu-grades-list");
    ul.innerHTML = grades.length === 0 ? '<li class="empty">Оценок пока нет.</li>' : "";
    for (const g of grades) {
      const li = document.createElement("li");
      li.innerHTML = `
        <div class="grade-row">
          <span class="grade-badge grade-${g.grade}">${g.grade}</span>
          <span class="item-title">${h(g.subject || "Без предмета")}</span>
        </div>
        <div class="item-meta">${new Date(g.submitted_at).toLocaleDateString("ru-RU")}</div>`;
      ul.appendChild(li);
    }
  } catch (err) { toast("Ошибка загрузки оценок: " + err.message, "error"); }
}

// Показываем текущий класс ученика
function showStudentClass(prof) {
  $("#stu-class-info").textContent = prof.class
    ? "Вы записаны в класс: " + prof.class
    : "Вы не привязаны ни к одному классу. Отправьте заявку.";
}

// Сдать работу
$("#submit-form").addEventListener("submit", async e => {
  e.preventDefault();
  const fd = new FormData(e.target);
  if (!fd.get("homework_id")) { toast("Выберите задание", "error"); return; }
  try {
    await api.createSubmission(fd);
    toast("Работа успешно сдана!");
    e.target.reset();
  } catch (err) { toast(err.message, "error"); }
});

// Обновить расписание по выбранной неделе
$("#stu-sched-load").addEventListener("click", loadStuSchedule);

// Заявка в класс
$("#request-form").addEventListener("submit", async e => {
  e.preventDefault();
  const cls = new FormData(e.target).get("class_name");
  try {
    await api.createRequest(cls);
    toast("Заявка отправлена! Ожидайте подтверждения.");
    e.target.reset();
  } catch (err) { toast(err.message, "error"); }
});

// =========================================================
// ===  TEACHER  ==========================================
// =========================================================

async function loadTeacherData() {
  await Promise.all([loadTeaHomeworks(), loadTeaRequests(), loadTeaSchedule()]);
}

// Расписание самого учителя (сеткой, во второй строке — класс)
async function loadTeaSchedule() {
  const wrap = $("#tea-sched-grid");
  const week = $("#tea-sched-week").value;
  try {
    const list = (await api.getMySchedule(week)) || [];
    const map = {};
    for (const s of list) map[`${s.day_of_week}_${s.lesson_num}`] = s;
    wrap.innerHTML = readonlyScheduleGridHTML(map, "teacher");
  } catch (err) { toast("Ошибка загрузки расписания: " + err.message, "error"); }
}
$("#tea-sched-load").addEventListener("click", loadTeaSchedule);

// Загрузить форму + список заданий учителя
$("#upload-form").addEventListener("submit", async e => {
  e.preventDefault();
  try {
    await api.uploadHomework(new FormData(e.target));
    toast("Задание загружено!");
    e.target.reset();
    await loadTeaHomeworks();
  } catch (err) { toast(err.message, "error"); }
});

async function loadTeaHomeworks() {
  try {
    const items = (await api.listHomeworks()) || [];

    // Список "Мои задания"
    const ul = $("#tea-hw-list");
    ul.innerHTML = items.length === 0 ? '<li class="empty">Вы ещё не загрузили ни одного задания.</li>' : "";

    // Обновляем селект в "Работы учеников"
    const sel  = $("#subs-hw-select");
    const prev = sel.value;
    sel.innerHTML = '<option value="">— выберите задание —</option>';

    for (const hw of items) {
      // Карточка
      const li = document.createElement("li");
      li.innerHTML = `
        <div class="item-title">${h(hw.filename)}</div>
        <div class="item-meta">${[
          hw.class_name  ? "Класс: "    + h(hw.class_name)  : null,
          hw.subject     ? "Предмет: "  + h(hw.subject)     : null,
          hw.description ? "Описание: " + h(hw.description) : null,
        ].filter(Boolean).join(" · ") || "—"}</div>
        <div class="item-actions">
          ${hw.filepath ? downloadLink(hw.filepath, "Скачать") : ""}
          <button class="btn-small" data-a="edit">Изменить</button>
          <button class="btn-small" data-a="replace">Заменить файл</button>
          <button class="btn-small btn-danger" data-a="del">Удалить</button>
          <button class="btn-outline" data-a="subs">Работы учеников</button>
        </div>`;
      li.querySelectorAll("[data-a]").forEach(btn =>
        btn.addEventListener("click", () => onTeaHwAction(btn.dataset.a, hw))
      );
      ul.appendChild(li);

      // Опция
      const opt = document.createElement("option");
      opt.value       = hw.id;
      opt.textContent = (hw.subject ? hw.subject + " — " : "") + hw.filename;
      sel.appendChild(opt);
    }
    sel.value = prev;
  } catch (err) { toast("Ошибка загрузки заданий: " + err.message, "error"); }
}

async function onTeaHwAction(action, hw) {
  if (action === "del") {
    if (!confirm(`Удалить "${hw.filename}"?`)) return;
    try {
      await api.deleteHomework(hw.id);
      toast("Задание удалено");
      await loadTeaHomeworks();
    } catch (err) { toast(err.message, "error"); }

  } else if (action === "edit") {
    const subject     = prompt("Предмет:",    hw.subject     || ""); if (subject     === null) return;
    const description = prompt("Описание:",   hw.description || ""); if (description === null) return;
    try {
      await api.updateHomework(hw.id, { subject, description });
      toast("Задание обновлено");
      await loadTeaHomeworks();
    } catch (err) { toast(err.message, "error"); }

  } else if (action === "replace") {
    const inp = document.createElement("input");
    inp.type = "file";
    inp.onchange = async () => {
      if (!inp.files.length) return;
      const fd = new FormData();
      fd.append("file",        inp.files[0]);
      fd.append("subject",     hw.subject     || "");
      fd.append("description", hw.description || "");
      try {
        await api.replaceHomework(hw.id, fd);
        toast("Файл заменён");
        await loadTeaHomeworks();
      } catch (err) { toast(err.message, "error"); }
    };
    inp.click();

  } else if (action === "subs") {
    $("#subs-hw-select").value = hw.id;
    switchPanel("teacher-section", "tea-submissions");
    await loadSubmissions(hw.id);
  }
}

// Кнопка "Показать" работы учеников
$("#subs-load-btn").addEventListener("click", async () => {
  const hwId = $("#subs-hw-select").value;
  if (!hwId) { toast("Выберите задание", "error"); return; }
  await loadSubmissions(hwId);
});

async function loadSubmissions(hwId) {
  subsCurrentHw = hwId; // запоминаем для авто-обновления
  try {
    const list = (await api.getSubmissions(hwId)) || [];
    const ul   = $("#subs-list");
    ul.innerHTML = list.length === 0 ? '<li class="empty">Работ пока нет.</li>' : "";

    for (const sb of list) {
      const li = document.createElement("li");
      const gradeHtml = sb.grade !== null
        ? `<span class="grade-badge grade-${sb.grade}">${sb.grade}</span>`
        : `<span class="chip chip--pending">На проверке</span>`;

      li.innerHTML = `
        <div class="grade-row">
          <span class="item-title">${h(sb.full_name || ("Студент #" + sb.student_id))}</span> ${gradeHtml}
        </div>
        <div class="item-meta">
          ${downloadLink(sb.filepath, "Скачать работу")} <span>· ${new Date(sb.submitted_at).toLocaleDateString("ru-RU")}</span>
        </div>
        ${sb.status === "pending" ? `
        <div class="item-actions">
          <input class="grade-input" type="number" min="1" max="5" placeholder="1–5">
          <button class="btn-outline" data-sub-id="${sb.id}">Выставить оценку</button>
        </div>` : ""}`;

      if (sb.status === "pending") {
        li.querySelector("[data-sub-id]").addEventListener("click", async () => {
          const grade = parseInt(li.querySelector(".grade-input").value, 10);
          if (!grade || grade < 1 || grade > 5) { toast("Оценка от 1 до 5", "error"); return; }
          try {
            await api.gradeSubmission(sb.id, grade);
            toast("Оценка выставлена!");
            await loadSubmissions(hwId); // перерисовываем список
          } catch (err) { toast(err.message, "error"); }
        });
      }
      ul.appendChild(li);
    }
  } catch (err) { toast("Ошибка загрузки работ: " + err.message, "error"); }
}

async function loadTeaRequests() {
  await loadRequests("tea-req-list");
}

// =========================================================
// ===  ADMIN  ============================================
// =========================================================

async function loadAdminData() {
  await Promise.all([loadTeachers(), loadAdmRequests(), loadUsers()]);
}

// Список пользователей + действия (кик из класса / удаление)
const ROLE_RU = { student: "Ученик", teacher: "Учитель", admin: "Админ" };
async function loadUsers() {
  try {
    const users = (await api.listUsers()) || [];
    const ul = $("#adm-users-list");
    ul.innerHTML = users.length === 0 ? '<li class="empty">Пользователей нет.</li>' : "";

    for (const u of users) {
      const meta = [
        ROLE_RU[u.role] || u.role,
        u.role === "student" && u.class ? "класс " + u.class : null,
        u.role === "student" && !u.class ? "без класса" : null,
        u.role === "teacher" && u.subject ? u.subject : null,
        "@" + u.username,
      ].filter(Boolean).join(" · ");

      const li = document.createElement("li");
      li.innerHTML = `
        <div class="item-title">${h(u.full_name)}</div>
        <div class="item-meta">${h(meta)}</div>
        <div class="item-actions">
          <button class="btn-small" data-act="pass">Сменить пароль</button>
          ${u.role === "student" && u.class ? `<button class="btn-small" data-act="kick">Исключить из класса</button>` : ""}
          ${u.role !== "admin" ? `<button class="btn-small btn-danger" data-act="del">Удалить аккаунт</button>` : `<span class="chip chip--neutral">админ</span>`}
        </div>`;

      const passBtn = li.querySelector('[data-act="pass"]');
      if (passBtn) passBtn.addEventListener("click", async () => {
        const np = prompt(`Новый пароль для «${u.full_name}» (@${u.username}), минимум 6 символов:`);
        if (np === null) return;
        if (np.trim().length < 6) { toast("Пароль минимум 6 символов", "error"); return; }
        try {
          await api.setPassword(u.id, np.trim());
          toast(`Пароль обновлён. Передайте: @${u.username} / ${np.trim()}`);
        } catch (err) { toast(err.message, "error"); }
      });

      const kickBtn = li.querySelector('[data-act="kick"]');
      if (kickBtn) kickBtn.addEventListener("click", async () => {
        if (!confirm(`Исключить ${u.full_name} из класса ${u.class}?`)) return;
        try { await api.kickUser(u.id); toast("Ученик исключён из класса"); await loadUsers(); }
        catch (err) { toast(err.message, "error"); }
      });

      const delBtn = li.querySelector('[data-act="del"]');
      if (delBtn) delBtn.addEventListener("click", async () => {
        if (!confirm(`Удалить аккаунт «${u.full_name}» безвозвратно? Все его данные тоже будут удалены.`)) return;
        try { await api.deleteUser(u.id); toast("Аккаунт удалён"); await loadUsers(); }
        catch (err) { toast(err.message, "error"); }
      });

      ul.appendChild(li);
    }

    // Заполняем список классов для меню в редакторе расписания
    const dl = $("#sched-class-list");
    if (dl) {
      const classes = [...new Set(users.filter(u => u.class).map(u => u.class))].sort();
      dl.innerHTML = classes.map(c => `<option value="${h(c)}">`).join("");
    }
  } catch (err) { toast("Ошибка загрузки пользователей: " + err.message, "error"); }
}

// =========================================================
// Редактор расписания (админ): сетка «уроки × дни» + редактор слота
// =========================================================
const DAYS_SHORT = ["", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"];
const SCHED_LESSONS = 7; // максимум уроков в день
let schedTeachers = [];  // кэш учителей для выпадающего списка
let schedState = { cls: "", week: "odd", map: {} }; // map: `${day}_${lesson}` -> урок
let schedEditing = null; // { day, lesson, id|null }

// Загружаем учителей в кэш (используется в редакторе слота)
async function loadTeachers() {
  try { schedTeachers = (await api.listTeachers()) || []; }
  catch (err) { toast("Ошибка загрузки учителей: " + err.message, "error"); }
}
function teacherNameById(id) {
  const t = schedTeachers.find(t => t.id === id);
  return t ? t.full_name : ("учитель #" + id);
}
function teacherOptionsHtml(selectedId) {
  let o = '<option value="">— выберите учителя —</option>';
  for (const t of schedTeachers) {
    o += `<option value="${t.id}" ${t.id === selectedId ? "selected" : ""}>${h(t.full_name)}${t.subject ? " (" + h(t.subject) + ")" : ""}</option>`;
  }
  return o;
}

// Открыть расписание выбранного класса/недели
async function openSchedule() {
  const cls = ($("#sched-class").value || "").trim().toUpperCase();
  if (!cls) { toast("Укажите класс", "error"); return; }
  $("#sched-class").value = cls;
  schedState.cls = cls;
  schedState.week = $("#sched-week").value;
  hideSchedEditor();
  await reloadScheduleGrid();
}

async function reloadScheduleGrid() {
  if (!schedState.cls) return;
  try {
    const list = (await api.getSchedule(schedState.cls, schedState.week)) || [];
    schedState.map = {};
    for (const s of list) schedState.map[`${s.day_of_week}_${s.lesson_num}`] = s;
    renderScheduleGrid();
  } catch (err) { toast("Ошибка загрузки расписания: " + err.message, "error"); }
}

function renderScheduleGrid() {
  const wrap = $("#sched-grid-wrap");
  const weekRu = schedState.week === "odd" ? "нечётная" : "чётная";
  let html = `<div class="sched-grid-title">Класс ${h(schedState.cls)} · ${weekRu} неделя</div>`;
  html += `<div class="table-wrap"><table class="sched-grid"><thead><tr><th>Урок</th>`;
  for (let d = 1; d <= 6; d++) html += `<th>${DAYS_SHORT[d]}</th>`;
  html += `</tr></thead><tbody>`;
  for (let l = 1; l <= SCHED_LESSONS; l++) {
    html += `<tr><th class="sched-grid__num">${l}</th>`;
    for (let d = 1; d <= 6; d++) {
      const les = schedState.map[`${d}_${l}`];
      if (les) {
        html += `<td class="sched-cell sched-cell--filled" data-day="${d}" data-lesson="${l}">
          <button class="sched-cell__del" data-del="${les.id}" title="Удалить">×</button>
          <div class="sched-cell__subj">${h(les.subject)}</div>
          <div class="sched-cell__meta">${h(teacherNameById(les.teacher_id))}${les.room ? " · каб. " + h(les.room) : ""}</div>
        </td>`;
      } else {
        html += `<td class="sched-cell sched-cell--empty" data-day="${d}" data-lesson="${l}"><span class="sched-cell__add">＋</span></td>`;
      }
    }
    html += `</tr>`;
  }
  html += `</tbody></table></div>`;
  wrap.innerHTML = html;

  wrap.querySelectorAll(".sched-cell").forEach(cell => {
    cell.addEventListener("click", e => {
      if (e.target.closest("[data-del]")) return;
      const day = +cell.dataset.day, lesson = +cell.dataset.lesson;
      openSchedEditor(day, lesson, schedState.map[`${day}_${lesson}`] || null);
    });
  });
  wrap.querySelectorAll("[data-del]").forEach(btn => {
    btn.addEventListener("click", async e => {
      e.stopPropagation();
      if (!confirm("Удалить этот урок?")) return;
      try { await api.deleteSchedule(+btn.dataset.del); toast("Урок удалён"); hideSchedEditor(); await reloadScheduleGrid(); }
      catch (err) { toast(err.message, "error"); }
    });
  });
}

function openSchedEditor(day, lesson, les) {
  schedEditing = { day, lesson, id: les ? les.id : null };
  const weekRu = schedState.week === "odd" ? "нечётная" : "чётная";
  $("#sched-editor-title").textContent =
    `${DAYS[day]}, урок ${lesson} · класс ${schedState.cls} · ${weekRu} неделя`;
  $("#se-teacher").innerHTML = teacherOptionsHtml(les ? les.teacher_id : 0);
  $("#se-subject").value = les ? les.subject : "";
  $("#se-room").value = les && les.room ? les.room : "";
  $("#se-delete").hidden = !les;
  $("#sched-editor").hidden = false;
  $("#se-subject").focus();
}
function hideSchedEditor() { $("#sched-editor").hidden = true; schedEditing = null; }

$("#sched-open").addEventListener("click", openSchedule);
$("#sched-week").addEventListener("change", () => { if (schedState.cls) openSchedule(); });
$("#se-cancel").addEventListener("click", hideSchedEditor);
$("#se-delete").addEventListener("click", async () => {
  if (!schedEditing || !schedEditing.id) return;
  if (!confirm("Удалить этот урок?")) return;
  try { await api.deleteSchedule(schedEditing.id); toast("Урок удалён"); hideSchedEditor(); await reloadScheduleGrid(); }
  catch (err) { toast(err.message, "error"); }
});
$("#se-save").addEventListener("click", async () => {
  if (!schedEditing) return;
  const subject = $("#se-subject").value.trim();
  const teacherId = parseInt($("#se-teacher").value, 10);
  const room = $("#se-room").value.trim();
  if (!subject) { toast("Укажите предмет", "error"); return; }
  if (!teacherId) { toast("Выберите учителя", "error"); return; }
  const data = {
    class_name:  schedState.cls,
    week_parity: schedState.week,
    day_of_week: schedEditing.day,
    lesson_num:  schedEditing.lesson,
    subject,
    teacher_id:  teacherId,
    room: room || null,
  };
  try {
    if (schedEditing.id) await api.updateSchedule(schedEditing.id, data);
    else                 await api.createSchedule(data);
    toast("Расписание сохранено");
    hideSchedEditor();
    await reloadScheduleGrid();
  } catch (err) { toast(err.message, "error"); }
});

async function loadAdmRequests() {
  await loadRequests("adm-req-list");
}

// =========================================================
// Рендер заявок — общий для учителя и админа
// =========================================================
async function loadRequests(listId) {
  try {
    const list = (await api.getRequests("pending")) || [];
    const ul   = document.getElementById(listId);
    ul.innerHTML = list.length === 0 ? '<li class="empty">Новых заявок нет.</li>' : "";

    for (const req of list) {
      const li = document.createElement("li");
      li.innerHTML = `
        <div class="item-title">${h(req.full_name)} <span class="hint">→ класс ${h(req.class_name)}</span></div>
        <div class="item-meta">${new Date(req.created_at).toLocaleDateString("ru-RU")}</div>
        <div class="item-actions">
          <button class="btn-outline"         data-rid="${req.id}" data-st="accepted">Принять</button>
          <button class="btn-small btn-danger" data-rid="${req.id}" data-st="rejected">Отклонить</button>
        </div>`;
      li.querySelectorAll("[data-rid]").forEach(btn => {
        btn.addEventListener("click", async () => {
          try {
            await api.decideRequest(req.id, btn.dataset.st);
            toast(btn.dataset.st === "accepted" ? "Заявка принята" : "Заявка отклонена");
            await loadRequests(listId);
          } catch (err) { toast(err.message, "error"); }
        });
      });
      ul.appendChild(li);
    }
  } catch (err) { toast("Ошибка загрузки заявок: " + err.message, "error"); }
}

// =========================================================
// BOOTSTRAP — запускается при загрузке страницы и после логина
// =========================================================

// Вешаем обработчики вкладок один раз при загрузке страницы
setupRoleTabs("student-section");
setupRoleTabs("teacher-section");
setupRoleTabs("admin-section");

// =========================================================
// Авто-обновление данных (чтобы пользователю не жать F5)
// =========================================================
function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
}
function startPolling() {
  stopPolling();
  pollTimer = setInterval(pollTick, 15000); // каждые 15 секунд
}

async function pollTick() {
  if (document.hidden) return; // вкладка не на экране — не дёргаем сервер
  const ae = document.activeElement; // не мешаем, пока пользователь печатает
  if (ae && /^(INPUT|SELECT|TEXTAREA)$/.test(ae.tagName)) return;
  if (!getToken()) { stopPolling(); return; }

  // Проверяем сессию; заодно ловим её истечение и обновляем класс ученика.
  let prof;
  try { prof = await api.profile(); }
  catch { stopPolling(); clearAuth(); showSection(null); return; }

  try {
    if (prof.role === "student") {
      studentClass = prof.class || "";
      await Promise.all([loadStuHomeworks(), loadStuGrades(), loadStuSchedule()]);
    } else if (prof.role === "teacher") {
      await Promise.all([loadTeaHomeworks(), loadTeaRequests()]);
      // "Работы учеников" обновляем, только если вкладка открыта и оценка не начата —
      // иначе затрём набранное в поле ввода.
      const subsActive = document.getElementById("tea-submissions").classList.contains("active");
      if (subsActive && subsCurrentHw) {
        const dirty = [...document.querySelectorAll("#subs-list .grade-input")].some(i => i.value.trim() !== "");
        if (!dirty) await loadSubmissions(subsCurrentHw);
      }
    } else if (prof.role === "admin") {
      await Promise.all([loadAdmRequests(), loadUsers()]);
    }
  } catch (_) { /* разовую ошибку сети при фоновом обновлении игнорируем */ }
}

async function bootstrap() {
  if (!getToken()) { stopPolling(); showSection(null); return; }
  try {
    const prof = await api.profile();
    const role = prof.role;
    localStorage.setItem("role",     role);
    localStorage.setItem("username", prof.username);

    $("#user-name").textContent       = prof.full_name || prof.username;
    $("#user-role-badge").textContent = ({ student: "Ученик", teacher: "Учитель", admin: "Админ" }[role] || role);
    $("#user-avatar").textContent     = (prof.full_name || prof.username || "U").trim().charAt(0);

    showSection(role);

    if      (role === "student") await loadStudentData(prof);
    else if (role === "teacher") await loadTeacherData();
    else if (role === "admin")   await loadAdminData();

    startPolling(); // включаем авто-обновление
  } catch (_) {
    stopPolling();
    clearAuth();
    showSection(null);
  }
}

bootstrap();
