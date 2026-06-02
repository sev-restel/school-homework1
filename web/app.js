// =========================================================
// Вспомогательные данные
// =========================================================
const DAYS = ["", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"];

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
  clearAuth();
  showSection(null);
  toast("Вы вышли из системы");
});

// =========================================================
// ===  STUDENT  ==========================================
// =========================================================

async function loadStudentData(prof) {
  showStudentClass(prof);
  // Грузим задания и оценки параллельно
  await Promise.all([loadStuHomeworks(), loadStuGrades()]);
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
          hw.subject     ? "Предмет: "  + h(hw.subject)     : null,
          hw.description ? "Описание: " + h(hw.description) : null,
        ].filter(Boolean).join(" · ") || "—"}</div>
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
  await Promise.all([loadTeaHomeworks(), loadTeaRequests()]);
}

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
          <span class="item-title">Студент #${sb.student_id}</span> ${gradeHtml}
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
  await Promise.all([loadTeachers(), loadAdmRequests()]);
}

// Заполняет выпадающий список учителей в форме расписания
async function loadTeachers() {
  try {
    const teachers = (await api.listTeachers()) || [];
    const sel = $("#schedule-teacher-select");
    sel.innerHTML = '<option value="">— выберите учителя —</option>';
    for (const t of teachers) {
      const opt = document.createElement("option");
      opt.value       = t.id;
      opt.textContent = t.full_name + (t.subject ? ` (${t.subject})` : "");
      sel.appendChild(opt);
    }
  } catch (err) { toast("Ошибка загрузки учителей: " + err.message, "error"); }
}

// Форма создания записи расписания
$("#schedule-form").addEventListener("submit", async e => {
  e.preventDefault();
  const fd = new FormData(e.target);
  const teacherId = parseInt(fd.get("teacher_id"), 10);
  if (!teacherId) { toast("Выберите учителя", "error"); return; }

  const room = fd.get("room").trim();
  const data = {
    class_name:  fd.get("class_name"),
    week_parity: fd.get("week_parity"),
    day_of_week: parseInt(fd.get("day_of_week"), 10),
    lesson_num:  parseInt(fd.get("lesson_num"),  10),
    subject:     fd.get("subject"),
    teacher_id:  teacherId,
    room:        room || null, // пустая строка → null (иначе сервер вернёт ошибку)
  };
  try {
    await api.createSchedule(data);
    toast("Запись добавлена в расписание!");
    e.target.reset();
  } catch (err) { toast(err.message, "error"); }
});

// Просмотр расписания
$("#sched-load-btn").addEventListener("click", async () => {
  const cls  = $("#sched-class-filter").value.trim();
  const week = $("#sched-week-filter").value;
  if (!cls) { toast("Введите класс", "error"); return; }
  try {
    const list  = (await api.getSchedule(cls, week)) || [];
    const tbody = $("#schedule-tbody");
    tbody.innerHTML = "";

    if (list.length === 0) {
      tbody.innerHTML = "<tr><td colspan='5' class='hint'>Записей нет.</td></tr>";
      return;
    }
    for (const s of list) {
      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td>${h(DAYS[s.day_of_week] || s.day_of_week)}</td>
        <td>${s.lesson_num}</td>
        <td>${h(s.subject)}</td>
        <td>${s.room ? h(s.room) : "—"}</td>
        <td><button class="btn-small btn-danger" data-sid="${s.id}">Удалить</button></td>`;
      tr.querySelector("[data-sid]").addEventListener("click", async () => {
        if (!confirm("Удалить запись из расписания?")) return;
        try {
          await api.deleteSchedule(s.id);
          toast("Запись удалена");
          $("#sched-load-btn").click(); // перезагружаем таблицу
        } catch (err) { toast(err.message, "error"); }
      });
      tbody.appendChild(tr);
    }
  } catch (err) { toast("Ошибка: " + err.message, "error"); }
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

async function bootstrap() {
  if (!getToken()) { showSection(null); return; }
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
  } catch (_) {
    clearAuth();
    showSection(null);
  }
}

bootstrap();
