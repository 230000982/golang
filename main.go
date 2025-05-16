package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/jung-kurt/gofpdf"
)

var db *sql.DB

var store = sessions.NewCookieStore([]byte("secret-key"))

// ----- MAIN ----- //

// START :)
func main() {
	initDB()
	defer db.Close()

	// ROTA PUBLICAS
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/auth", loginHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/register", registerHandler)

	// ROTA PROTEGIDAS
	http.HandleFunc("/concursos", authMiddleware(concursosHandler))
	http.HandleFunc("/edit-concurso/", authMiddleware(editConcursoHandler))
	http.HandleFunc("/update-concurso/", authMiddleware(updateConcursoHandler))
	http.HandleFunc("/create-concurso", authMiddleware(createConcursoHandler))
	http.HandleFunc("/save-concurso", authMiddleware(saveConcursoHandler))
	http.HandleFunc("/concursos-ordenados", authMiddleware(concursosOrderByHandler))
	http.HandleFunc("/download-pdf", authMiddleware(downloadPDFHandler))

	// PORTA 8080
	fmt.Println("SV PORT 8080")
	http.ListenAndServe(":8080", nil)
}

// ----- // ----- // ----- //

// ----- MAIL ----- SYSTEM ----- //
func sendMail(messageBody string) {
	// SENDER INFO
	from := "notbeso2000@gmail.com"
	password := "wmfhdtxnsegwhnzj"

	// GET RECEIVERS FROM DB
	to, err := getEmailsFromDB()
	if err != nil {
		fmt.Println("Error getting emails from database:", err)
		return
	}

	if len(to) == 0 {
		fmt.Println("No recipients found in database")
		return
	}

	// SMTP SERVER INFO
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	// MESSAGE
	message := []byte(messageBody)

	// AUTHENTICATION
	auth := smtp.PlainAuth("", from, password, smtpHost)

	// SEND EMAIL
	err = smtp.SendMail(smtpHost+":"+smtpPort, auth, from, to, message)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Email Sent Successfully to all recipients!")
}

// Vai buscar os emails na DB
// (ID 1 é admin e ID 4 é gueste os mais são apenas enviados para o ID 2 e 3 (SAV e DCP))
func getEmailsFromDB() ([]string, error) {
	var emails []string

	rows, err := db.Query(`
        SELECT email FROM user 
        WHERE cargo_id NOT IN (1, 4) OR cargo_id IS NULL
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return emails, nil
}

// ----- // ----- // ----- //

// ----- DB ----- //
func initDB() {
	var err error
	//CONFIG DB
	db, err = sql.Open("mysql", "beso:beso@tcp(127.0.0.1:3306)/db")
	if err != nil {
		log.Fatal(err)
	}
	//CONN DB
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("DB OK")
}

// ----- // ----- // ----- //

// ----- AUTH ----- //

// falta diferenciar cargos (Admin, SAV, DCP e Guest) (60% ok)
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// CHECK AUTH
		session, _ := store.Get(r, "session-name")
		if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		// NEXT HANDLER
		next(w, r)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		password := r.FormValue("password")

		var storedPassword string
		var id_user int
		var cargo int
		err := db.QueryRow("SELECT id_user, password, cargo_id FROM user WHERE email = ?", email).Scan(&id_user, &storedPassword, &cargo)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Email não encontrado", http.StatusUnauthorized)
			} else {
				http.Error(w, "Erro 452", http.StatusInternalServerError)
			}
			return
		}

		// Comparar a senha fornecida com o hash da db
		err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password))
		if err != nil {
			http.Error(w, "Password incorreta", http.StatusUnauthorized)
			return
		}

		// Criar sessão
		session, _ := store.Get(r, "session-name")
		session.Values["authenticated"] = true
		session.Values["user_id"] = id_user
		session.Values["cargo"] = cargo
		session.Save(r, w)

		http.Redirect(w, r, "/index", http.StatusSeeOther)
	} else {
		tmpl := template.Must(template.ParseFiles("templates/auth.html"))
		tmpl.Execute(w, nil)
	}
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm-password")

		// Validações básicas
		if password != confirmPassword {
			http.Error(w, "As senhas não coincidem", http.StatusBadRequest)
			return
		}

		// Verificar se o email já existe
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM user WHERE email = ?", email).Scan(&count)
		if err != nil {
			http.Error(w, "Error 457", http.StatusInternalServerError)
			return
		}
		if count > 0 {
			http.Error(w, "Email já em uso", http.StatusBadRequest)
			return
		}

		// Hash da password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Error 483", http.StatusInternalServerError)
			return
		}

		// Inserir novo usuário (cargo_id - 1 = Admin, 2 = SAV, 3 = DCP, 4 = Guest)
		_, err = db.Exec("INSERT INTO user (nome, email, password, cargo_id) VALUES (?, ?, ?, 1)", "user", email, string(hashedPassword))
		if err != nil {
			http.Error(w, "Error 412", http.StatusInternalServerError)
			return
		}

		// Redirecionar para login após registro bem-sucedido
		http.Redirect(w, r, "/login?registered=true", http.StatusSeeOther)
	} else {
		// Se não for POST, mostrar o template normalmente
		tmpl := template.Must(template.ParseFiles("templates/auth.html"))
		tmpl.Execute(w, nil)
	}
}

// ----- // ----- // ----- //

// ----- HANDLERS ----- //

// Preparar a pagina inicial (100% ok)
// precisa de um layout visualmente bonito
func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	tmpl.Execute(w, nil)
}

// Preparar a pagina para editar um concurso (100% ok)
func editConcursoHandler(w http.ResponseWriter, r *http.Request) {
	// ID CONCURSO URL
	idConcurso := strings.TrimPrefix(r.URL.Path, "/edit-concurso/")
	if idConcurso == "" {
		http.Error(w, "ID do concurso inválido", http.StatusBadRequest)
		return
	}

	// SELECT CONCURSO ON DB
	var concurso struct {
		ID            int
		Preco         float64
		Referencia    string
		Entidade      string
		DiaErro       sql.NullString
		HoraErro      sql.NullString
		DiaProposta   sql.NullString
		HoraProposta  sql.NullString
		ReferenciaBC  string
		Preliminar    bool
		DiaAudiencia  sql.NullString
		HoraAudiencia sql.NullString
		Final         bool
		Recurso       bool
		Impugnacao    bool
		TipoID        int
		PlataformaID  int
		EstadoID      int
	}

	err := db.QueryRow(`
        SELECT c.id_concurso, c.preco, c.referencia, c.entidade, c.dia_erro, c.hora_erro, 
               c.dia_proposta, c.hora_proposta, c.referencia_bc, c.preliminar, 
               c.dia_audiencia, c.hora_audiencia, c.final, c.recurso, c.impugnacao, 
               c.tipo_id, c.plataforma_id, c.estado_id
        FROM concurso c
        WHERE c.id_concurso = ?
    `, idConcurso).Scan(
		&concurso.ID, &concurso.Preco, &concurso.Referencia, &concurso.Entidade,
		&concurso.DiaErro, &concurso.HoraErro, &concurso.DiaProposta, &concurso.HoraProposta,
		&concurso.ReferenciaBC, &concurso.Preliminar, &concurso.DiaAudiencia, &concurso.HoraAudiencia,
		&concurso.Final, &concurso.Recurso, &concurso.Impugnacao,
		&concurso.TipoID, &concurso.PlataformaID, &concurso.EstadoID,
	)
	if err != nil {
		http.Error(w, "Erro ao buscar concurso", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// SELECT RELATED DATA
	tipos, err := getTipos()
	if err != nil {
		http.Error(w, "Erro ao buscar tipos", http.StatusInternalServerError)
		return
	}

	plataformas, err := getPlataformas()
	if err != nil {
		http.Error(w, "Erro ao buscar plataformas", http.StatusInternalServerError)
		return
	}

	estados, err := getEstados()
	if err != nil {
		http.Error(w, "Erro ao buscar estados", http.StatusInternalServerError)
		return
	}

	resultado, err := getAdjudicatario()
	if err != nil {
		http.Error(w, "Erro ao buscar estados", http.StatusInternalServerError)
		return
	}

	// PREPARE DATA
	data := struct {
		Concurso interface{}
		Tipos    []struct {
			ID        int
			Descricao string
		}
		Plataformas []struct {
			ID        int
			Descricao string
		}
		Estados []struct {
			ID        int
			Descricao string
		}
		Resultado []struct {
			ID        int
			Descricao string
		}
	}{
		Concurso:    concurso,
		Tipos:       tipos,
		Plataformas: plataformas,
		Estados:     estados,
		Resultado:   resultado,
	}

	// RENDER TEMPLATE EDITAR_CONCURSO
	tmpl := template.Must(template.ParseFiles("templates/edit_concurso.html"))
	tmpl.Execute(w, data)
}

// Atualiza o concurso na DB (90% ok)
// (precisa de enviar um mail com a atualização)
func updateConcursoHandler(w http.ResponseWriter, r *http.Request) {
	// ID CONCURSO URL
	idConcurso := strings.TrimPrefix(r.URL.Path, "/update-concurso/")
	if idConcurso == "" {
		http.Error(w, "ID do concurso inválido", http.StatusBadRequest)
		return
	}

	// PROCESS FORM
	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Erro ao processar formulário", http.StatusBadRequest)
			return
		}

		preco := r.FormValue("preco")
		referencia := r.FormValue("referencia")
		entidade := r.FormValue("entidade")
		referenciaBC := r.FormValue("referencia_bc")
		diaErro := r.FormValue("dia_erro")
		horaErro := r.FormValue("hora_erro")
		diaProposta := r.FormValue("dia_proposta")
		horaProposta := r.FormValue("hora_proposta")
		diaAudiencia := r.FormValue("dia_audiencia")
		horaAudiencia := r.FormValue("hora_audiencia")
		preliminar := r.FormValue("preliminar") == "on"
		final := r.FormValue("final") == "on"
		recurso := r.FormValue("recurso") == "on"
		impugnacao := r.FormValue("impugnacao") == "on"
		tipo := r.FormValue("tipo_id")
		plataforma := r.FormValue("plataforma_id")
		estado := r.FormValue("estado_id")

		// UPDATE DB
		_, err = db.Exec(`
            UPDATE concurso
            SET preco = ?, referencia = ?, entidade = ?, referencia_bc = ?,
                dia_erro = ?, hora_erro = ?, dia_proposta = ?, hora_proposta = ?,
                dia_audiencia = ?, hora_audiencia = ?, preliminar = ?, final = ?,
                recurso = ?, impugnacao = ?, tipo_id = ?, plataforma_id = ?, estado_id = ?
            WHERE id_concurso = ?
        `,
			preco, referencia, entidade, referenciaBC,
			parseNullString(diaErro), parseNullString(horaErro),
			parseNullString(diaProposta), parseNullString(horaProposta),
			parseNullString(diaAudiencia), parseNullString(horaAudiencia),
			preliminar, final, recurso, impugnacao,
			tipo, plataforma, estado, idConcurso)

		if err != nil {
			http.Error(w, "Erro ao atualizar concurso", http.StatusInternalServerError)
			log.Println(err)
			return
		}

		// SENDMAIL
		message := "Um concurso foi atualizado:"
		switch estado {
		case "1": // 2
			message = "Em andamento"
		case "2":
			message = "Foi enviado"
		case "3":
			message = "Não foi enviado"
		case "4":
			message = "Declaração"
		}
		sendMail(message)

		fmt.Println("Update 100% ok")
		// REDIRECT TO CONCURSOS LIST
		http.Redirect(w, r, "/concursos", http.StatusSeeOther)
	}
}

// Preparar a pagina para criar um concuros (100% ok)
func createConcursoHandler(w http.ResponseWriter, r *http.Request) {
	// SELECT RELATED DATA
	tipos, err := getTipos()
	if err != nil {
		http.Error(w, "Erro ao buscar tipos", http.StatusInternalServerError)
		return
	}

	plataformas, err := getPlataformas()
	if err != nil {
		http.Error(w, "Erro ao buscar plataformas", http.StatusInternalServerError)
		return
	}

	estados, err := getEstados()
	if err != nil {
		http.Error(w, "Erro ao buscar estados", http.StatusInternalServerError)
		return
	}

	resultado, err := getAdjudicatario()
	if err != nil {
		http.Error(w, "Erro ao buscar resultado", http.StatusInternalServerError)
		return
	}

	// PREPARE DATA
	data := struct {
		Tipos []struct {
			ID        int
			Descricao string
		}
		Plataformas []struct {
			ID        int
			Descricao string
		}
		Estados []struct {
			ID        int
			Descricao string
		}
		Resultado []struct {
			ID        int
			Descricao string
		}
	}{
		Tipos:       tipos,
		Plataformas: plataformas,
		Estados:     estados,
		Resultado:   resultado,
	}

	// RENDER CRIAR_CONCURSO
	tmpl := template.Must(template.ParseFiles("templates/create_concurso.html"))
	tmpl.Execute(w, data)
}

// Cria o concurso na DB (90% ok)
// (precisa de enviar um mail com a criação)
func saveConcursoHandler(w http.ResponseWriter, r *http.Request) {
	// PROCESS FORM
	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Erro ao processar formulário", http.StatusBadRequest)
			return
		}

		// GET FORM VALUES
		referencia := r.FormValue("referencia")
		entidade := r.FormValue("entidade")
		precoStr := r.FormValue("preco")
		tipoIDStr := r.FormValue("tipo_id")
		plataformaIDStr := r.FormValue("plataforma_id")
		estadoIDStr := r.FormValue("estado_id")
		referenciaBC := r.FormValue("referencia_bc")
		diaErro := parseNullString(r.FormValue("dia_erro"))
		horaErro := parseNullString(r.FormValue("hora_erro"))
		diaProposta := parseNullString(r.FormValue("dia_proposta"))
		horaProposta := parseNullString(r.FormValue("hora_proposta"))
		diaAudiencia := parseNullString(r.FormValue("dia_audiencia"))
		horaAudiencia := parseNullString(r.FormValue("hora_audiencia"))
		preliminar := r.FormValue("preliminar") == "on"
		final := r.FormValue("final") == "on"
		recurso := r.FormValue("recurso") == "on"
		impugnacao := r.FormValue("impugnacao") == "on"

		// CONVERT STRINGS TO THEIR RESPECTIVE TYPES WITH DEFAULT VALUES
		var preco float64
		if precoStr != "" {
			preco, err = strconv.ParseFloat(precoStr, 64)
			if err != nil {
				http.Error(w, "Preço inválido", http.StatusBadRequest)
				return
			}
		}

		var tipoID int
		if tipoIDStr != "" {
			tipoID, err = strconv.Atoi(tipoIDStr)
			if err != nil {
				http.Error(w, "Tipo inválido", http.StatusBadRequest)
				return
			}
		}

		var plataformaID int
		if plataformaIDStr != "" {
			plataformaID, err = strconv.Atoi(plataformaIDStr)
			if err != nil {
				http.Error(w, "Plataforma inválida", http.StatusBadRequest)
				return
			}
		}

		var estadoID int
		if estadoIDStr != "" {
			estadoID, err = strconv.Atoi(estadoIDStr)
			if err != nil {
				http.Error(w, "Estado inválido", http.StatusBadRequest)
				return
			}
		}

		// INSERT DB
		_, err = db.Exec(`
            INSERT INTO concurso (
                referencia, entidade, dia_erro, hora_erro, 
                dia_proposta, hora_proposta, preco, tipo_id, 
                plataforma_id, referencia_bc, preliminar, dia_audiencia, 
                hora_audiencia, final, recurso, impugnacao, estado_id
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `,
			referencia, entidade, diaErro, horaErro, diaProposta, horaProposta,
			preco, tipoID, plataformaID, referenciaBC, preliminar, diaAudiencia,
			horaAudiencia, final, recurso, impugnacao, estadoID)

		if err != nil {
			http.Error(w, "Erro ao criar concurso: "+err.Error(), http.StatusInternalServerError)
			log.Println("Erro no INSERT:", err)
			return
		}

		// REDIRECT TO CONCURSOS LIST
		http.Redirect(w, r, "/concursos", http.StatusSeeOther)
	}
}

// Prepara a pagina para mostrar os concursos (100% ok)
func concursosHandler(w http.ResponseWriter, r *http.Request) {
	// Obter parâmetro de pesquisa da URL
	entidade := r.URL.Query().Get("entidade")

	// Construir a consulta SQL base
	query := `
        SELECT c.id_concurso, c.preco, c.referencia, c.entidade, c.dia_erro, c.hora_erro, c.dia_proposta, c.hora_proposta, 
               c.referencia_bc, c.preliminar, c.dia_audiencia, c.hora_audiencia, c.final, c.recurso, c.impugnacao, 
               t.descricao AS tipo, p.descricao AS plataforma, c.estado_id AS estado
        FROM concurso c
        JOIN tipo t ON c.tipo_id = t.id_tipo
        JOIN plataforma p ON c.plataforma_id = p.id_platforma
    `

	// Adicionar filtro se houver pesquisa por referência
	var args []interface{}
	if entidade != "" {
		query += " WHERE c.entidade LIKE ?"
		args = append(args, "%"+entidade+"%")
	}

	// Adicionar ordenação por dia_proposta e hora_proposta
	query += " ORDER BY c.dia_proposta DESC, c.hora_proposta DESC"

	// Executar a consulta
	rows, err := db.Query(query, args...)
	if err != nil {
		http.Error(w, "Erro ao buscar concursos", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer rows.Close()

	// PREPARE DATA
	type Concurso struct {
		ID            int
		Preco         string
		Referencia    string
		Entidade      string
		DiaErro       sql.NullString
		HoraErro      sql.NullString
		DiaProposta   sql.NullString
		HoraProposta  sql.NullString
		ReferenciaBC  string
		Preliminar    bool
		DiaAudiencia  sql.NullString
		HoraAudiencia sql.NullString
		Final         bool
		Recurso       bool
		Impugnacao    bool
		Tipo          string
		Plataforma    string
		Estado        int
	}

	var concursos []Concurso

	// SCAN ROWS
	for rows.Next() {
		var c Concurso
		var preco float64
		err := rows.Scan(
			&c.ID, &preco, &c.Referencia, &c.Entidade, &c.DiaErro, &c.HoraErro,
			&c.DiaProposta, &c.HoraProposta, &c.ReferenciaBC, &c.Preliminar,
			&c.DiaAudiencia, &c.HoraAudiencia, &c.Final, &c.Recurso, &c.Impugnacao,
			&c.Tipo, &c.Plataforma, &c.Estado,
		)
		if err != nil {
			http.Error(w, "Erro ao ler concursos", http.StatusInternalServerError)
			log.Println(err)
			return
		}

		c.Preco = fmt.Sprintf("%.2f€", preco)
		concursos = append(concursos, c)
	}

	// RENDER TEMPLATE CONCURSOS
	tmpl := template.Must(template.ParseFiles("templates/concursos.html"))
	tmpl.Execute(w, concursos)
}

// Ordena os concursos por data e hora (100% ok)
func concursosOrderByHandler(w http.ResponseWriter, r *http.Request) {
	// Obter a data e hora atual
	now := time.Now()
	currentDate := now.Format("2006-01-02")
	currentTime := now.Format("15:04:05")

	// SELECT CONCURSOS ON DB com filtro de estado
	rows, err := db.Query(`
        SELECT c.id_concurso, c.entidade, c.estado_id,
               c.dia_erro, c.hora_erro, 
               c.dia_proposta, c.hora_proposta,
               c.dia_audiencia, c.hora_audiencia,
			   c.tipo_id, c.referencia
        FROM concurso c
        WHERE c.estado_id = 1
        ORDER BY 
            COALESCE(c.dia_proposta, c.dia_erro, c.dia_audiencia) ASC,
            COALESCE(c.hora_proposta, c.hora_erro, c.hora_audiencia) ASC
    `)

	if err != nil {
		http.Error(w, "Erro ao buscar concursos", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer rows.Close()

	// Estrutura para os dados do concurso
	type ConcursoItem struct {
		Entidade   string
		Data       string
		Hora       string
		Tipo       string // "Proposta", "Erro" ou "Audiencia"
		Objeto     int    // "CTE", "CO", "INF", "CI" ou "RO"
		Referencia string
	}

	var items []ConcursoItem

	// SCAN ROWS
	for rows.Next() {
		var id, estado int
		var entidade string
		var diaErro, horaErro sql.NullString
		var diaProposta, horaProposta sql.NullString
		var diaAudiencia, horaAudiencia sql.NullString
		var referencia string
		var objeto int

		err := rows.Scan(
			&id, &entidade, &estado,
			&diaErro, &horaErro,
			&diaProposta, &horaProposta,
			&diaAudiencia, &horaAudiencia,
			&objeto, &referencia,
		)
		if err != nil {
			http.Error(w, "Erro ao ler concursos", http.StatusInternalServerError)
			log.Println(err)
			return
		}

		// Função auxiliar para verificar se a data já passou
		isFutureDate := func(date, time string) bool {
			if date > currentDate {
				return true
			}
			if date == currentDate && time > currentTime {
				return true
			}
			return false
		}

		// Adiciona cada data válida como um item separado, apenas se for futura
		if diaProposta.Valid && horaProposta.Valid && isFutureDate(diaProposta.String, horaProposta.String) {
			items = append(items, ConcursoItem{
				Entidade:   entidade,
				Data:       diaProposta.String,
				Hora:       horaProposta.String,
				Tipo:       "Proposta",
				Objeto:     objeto,
				Referencia: referencia,
			})
		}

		if diaErro.Valid && horaErro.Valid && isFutureDate(diaErro.String, horaErro.String) {
			items = append(items, ConcursoItem{
				Entidade:   entidade,
				Data:       diaErro.String,
				Hora:       horaErro.String,
				Tipo:       "Erro",
				Objeto:     objeto,
				Referencia: referencia,
			})
		}

		if diaAudiencia.Valid && horaAudiencia.Valid && isFutureDate(diaAudiencia.String, horaAudiencia.String) {
			items = append(items, ConcursoItem{
				Entidade:   entidade,
				Data:       diaAudiencia.String,
				Hora:       horaAudiencia.String,
				Tipo:       "Audiencia",
				Objeto:     objeto,
				Referencia: referencia,
			})
		}
	}

	// Ordena os itens por data e hora (doq termina primeiro para o que termina depois)
	sort.Slice(items, func(i, j int) bool {
		// Compara primeiro a data
		if items[i].Data != items[j].Data {
			return items[i].Data < items[j].Data
		}
		// Se a data for igual, compara a hora
		return items[i].Hora < items[j].Hora
	})

	// Carrega o template do arquivo
	tmpl, err := template.ParseFiles("templates/concursos_ordenados.html")
	if err != nil {
		http.Error(w, "Erro ao carregar template", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Dados para o template
	data := struct {
		Items       []ConcursoItem
		CurrentTime string
	}{
		Items:       items,
		CurrentTime: now.Format("2006-01-02 15:04:05"),
	}

	// Executa o template
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "Erro ao renderizar template", http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

// Gera o PDF com os concursos (100% ok)
func downloadPDFHandler(w http.ResponseWriter, r *http.Request) {
	// Obter a data e hora atual
	now := time.Now()
	currentDate := now.Format("2006-01-02")
	currentTime := now.Format("15:04:05")

	rows, err := db.Query(`
        SELECT c.id_concurso, c.entidade, c.estado_id,
               c.dia_erro, c.hora_erro, 
               c.dia_proposta, c.hora_proposta,
               c.dia_audiencia, c.hora_audiencia,
               c.tipo_id, c.referencia
        FROM concurso c
        WHERE c.estado_id = 1
        ORDER BY 
            COALESCE(c.dia_proposta, c.dia_erro, c.dia_audiencia) ASC,
            COALESCE(c.hora_proposta, c.hora_erro, c.hora_audiencia) ASC
    `)
	if err != nil {
		http.Error(w, "Erro ao buscar concursos", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer rows.Close()

	type ConcursoItem struct {
		Referencia string
		Entidade   string
		Objeto     int
		Data       string
		Hora       string
		Tipo       string
	}

	var items []ConcursoItem

	for rows.Next() {
		var id, estado, objeto int
		var entidade, referencia string
		var diaErro, horaErro sql.NullString
		var diaProposta, horaProposta sql.NullString
		var diaAudiencia, horaAudiencia sql.NullString

		err := rows.Scan(
			&id, &entidade, &estado,
			&diaErro, &horaErro,
			&diaProposta, &horaProposta,
			&diaAudiencia, &horaAudiencia,
			&objeto, &referencia,
		)
		if err != nil {
			log.Println("Erro no Scan:", err)
			continue
		}
		// Função auxiliar para verificar se a data já passou
		isFutureDate := func(date, time string) bool {
			if date > currentDate {
				return true
			}
			if date == currentDate && time > currentTime {
				return true
			}
			return false
		}

		if diaProposta.Valid && horaProposta.Valid && isFutureDate(diaProposta.String, horaProposta.String) {
			items = append(items, ConcursoItem{
				Referencia: referencia,
				Entidade:   entidade,
				Objeto:     objeto,
				Data:       diaProposta.String,
				Hora:       horaProposta.String,
				Tipo:       "Proposta",
			})
		}

		if diaErro.Valid && horaErro.Valid && isFutureDate(diaErro.String, horaErro.String) {
			items = append(items, ConcursoItem{
				Referencia: referencia,
				Entidade:   entidade,
				Objeto:     objeto,
				Data:       diaErro.String,
				Hora:       horaErro.String,
				Tipo:       "Erro",
			})
		}

		if diaAudiencia.Valid && horaAudiencia.Valid && isFutureDate(diaAudiencia.String, horaAudiencia.String) {
			items = append(items, ConcursoItem{
				Referencia: referencia,
				Entidade:   entidade,
				Objeto:     objeto,
				Data:       diaAudiencia.String,
				Hora:       horaAudiencia.String,
				Tipo:       "Audiencia",
			})
		}
	}

	// Ordenar os itens por data e hora do mais velho para o mais novo
	sort.Slice(items, func(i, j int) bool {
		dataHoraI := items[i].Data + " " + items[i].Hora
		dataHoraJ := items[j].Data + " " + items[j].Hora

		tI, errI := time.Parse("2006-01-02 15:04", dataHoraI)
		tJ, errJ := time.Parse("2006-01-02 15:04", dataHoraJ)

		if errI != nil || errJ != nil {
			return false
		}

		return tI.Before(tJ)
	})

	// Criar PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Configurações do PDF
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Concursos Futuros")
	pdf.Ln(12)

	// Cabeçalho da tabela
	pdf.SetFont("Arial", "B", 10)
	widths := []float64{40, 40, 20, 30, 20, 30}
	headers := []string{"REF.", "ENTIDADE", "OBJETO", "DATA", "HORA", "TIPO"}

	for i, header := range headers {
		pdf.CellFormat(widths[i], 10, header, "1", 0, "", false, 0, "")
	}
	pdf.Ln(-1)

	// Dados da tabela
	pdf.SetFont("Arial", "", 10)
	for _, item := range items {
		objetoStr := map[int]string{
			1: "CTE",
			2: "CON",
			3: "INF",
			4: "CI",
			5: "ROB",
		}[item.Objeto]

		pdf.CellFormat(widths[0], 10, item.Referencia, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[1], 10, item.Entidade, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[2], 10, objetoStr, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[3], 10, item.Data, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[4], 10, item.Hora, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[5], 10, item.Tipo, "1", 0, "", false, 0, "")
		pdf.Ln(-1)
	}

	// Rodapé
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(0, 10, fmt.Sprintf("Atualizado em: %s", now.Format("2006-01-02 15:04:05")))

	// Enviar PDF
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=concursos.pdf")

	if err := pdf.Output(w); err != nil {
		http.Error(w, "Erro ao gerar PDF", http.StatusInternalServerError)
		log.Println(err)
	}
}

// ----- // ----- // ----- //

//----- FUNCS ----- SUPORT -----//

// null strings
func parseNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// FUNC GET TIPO (100% ok)
func getTipos() ([]struct {
	ID        int
	Descricao string
}, error) {
	rows, err := db.Query("SELECT id_tipo, descricao FROM tipo")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tipos []struct {
		ID        int
		Descricao string
	}
	for rows.Next() {
		var t struct {
			ID        int
			Descricao string
		}
		err := rows.Scan(&t.ID, &t.Descricao)
		if err != nil {
			return nil, err
		}
		tipos = append(tipos, t)
	}
	return tipos, nil
}

// FUNC GET PLATAFORMAS (100% ok)
func getPlataformas() ([]struct {
	ID        int
	Descricao string
}, error) {
	rows, err := db.Query("SELECT id_platforma, descricao FROM plataforma")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plataformas []struct {
		ID        int
		Descricao string
	}
	for rows.Next() {
		var p struct {
			ID        int
			Descricao string
		}
		err := rows.Scan(&p.ID, &p.Descricao)
		if err != nil {
			return nil, err
		}
		plataformas = append(plataformas, p)
	}
	return plataformas, nil
}

// FUNC GET ESTADOS (100% ok)
func getEstados() ([]struct {
	ID        int
	Descricao string
}, error) {
	rows, err := db.Query("SELECT id_estado, descricao FROM estado")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var estados []struct {
		ID        int
		Descricao string
	}
	for rows.Next() {
		var e struct {
			ID        int
			Descricao string
		}
		err := rows.Scan(&e.ID, &e.Descricao)
		if err != nil {
			return nil, err
		}
		estados = append(estados, e)
	}
	return estados, nil
}

func getAdjudicatario() ([]struct {
	ID        int
	Descricao string
}, error) {
	rows, err := db.Query("SELECT id_resultado, descricao FROM resultado")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resultado []struct {
		ID        int
		Descricao string
	}
	for rows.Next() {
		var a struct {
			ID        int
			Descricao string
		}
		err := rows.Scan(&a.ID, &a.Descricao)
		if err != nil {
			return nil, err
		}
		resultado = append(resultado, a)
	}
	return resultado, nil
}

//----- FUNCS ----- SUPORT -----//

// ----- // ----- // ----- //

// ----- TESTES ----- //
