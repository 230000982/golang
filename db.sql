CREATE TABLE cargo (
    id_cargo INT PRIMARY KEY AUTO_INCREMENT,
    descricao TEXT NOT NULL
);

CREATE TABLE user (
    id_user INT PRIMARY KEY AUTO_INCREMENT,
    nome VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    cargo_id INT,
    FOREIGN KEY (cargo_id) REFERENCES cargo(id_cargo)
);

CREATE TABLE tipo (
    id_tipo INT PRIMARY KEY AUTO_INCREMENT,
    descricao VARCHAR(255) NOT NULL
);

CREATE TABLE plataforma (
    id_platforma INT PRIMARY KEY AUTO_INCREMENT,
    descricao VARCHAR(255) NOT NULL
);

CREATE TABLE estado (
    id_estado INT PRIMARY KEY AUTO_INCREMENT,
    descricao VARCHAR(255) NOT NULL
);

CREATE TABLE logs (
    id_logs INT PRIMARY KEY AUTO_INCREMENT,
    tabela VARCHAR(255) NOT NULL,
    acao VARCHAR(255) NOT NULL,
    old_data TEXT,
    new_data TEXT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    id_user INT,
    FOREIGN KEY (id_user) REFERENCES user(id_user)
);

CREATE TABLE concurso (
    id_concurso INT PRIMARY KEY AUTO_INCREMENT,
    referencia VARCHAR(255),
    entidade VARCHAR(255),
    dia_erro VARCHAR(255),
    hora_erro VARCHAR(255),
    dia_proposta VARCHAR(255),
    hora_proposta VARCHAR(255),
    preco DECIMAL(11, 2),
    tipo_id INT,
    plataforma_id INT,
    referencia_bc VARCHAR(255),
    preliminar BOOLEAN,   
    dia_audiencia VARCHAR(255),
    hora_audiencia VARCHAR(255),
    final BOOLEAN,
    recurso BOOLEAN,
    impugnacao BOOLEAN,
    estado_id INT,
    FOREIGN KEY (tipo_id) REFERENCES tipo(id_tipo),
    FOREIGN KEY (plataforma_id) REFERENCES plataforma(id_platforma),
    FOREIGN KEY (estado_id) REFERENCES estado(id_estado)
);

INSERT INTO cargo (id_cargo, descricao) VALUES (1, 'admin');
INSERT INTO cargo (id_cargo, descricao) VALUES (2, 'SAV');
INSERT INTO cargo (id_cargo, descricao) VALUES (3, 'DCP');
INSERT INTO cargo (id_cargo, descricao) VALUES (4, 'GUEST');

INSERT INTO tipo (id_tipo, descricao) VALUES (1, 'CTE');
INSERT INTO tipo (id_tipo, descricao) VALUES (2, 'CO');
INSERT INTO tipo (id_tipo, descricao) VALUES (3, 'INF');
INSERT INTO tipo (id_tipo, descricao) VALUES (4, 'CI');

INSERT INTO plataforma (id_platforma, descricao) VALUES (1, 'email');
INSERT INTO plataforma (id_platforma, descricao) VALUES (2, 'vortal');
INSERT INTO plataforma (id_platforma, descricao) VALUES (3, 'acingov');
INSERT INTO plataforma (id_platforma, descricao) VALUES (4, 'anogov');
INSERT INTO plataforma (id_platforma, descricao) VALUES (5, 'saphety');

INSERT INTO user (nome, email, password, cargo_id) 
VALUES ('beso', 'beso', 'beso', 1);

INSERT INTO estado (id_estado, descricao) VALUES (1, 'ongoing');
INSERT INTO estado (id_estado, descricao) VALUES (2, 'sent');
INSERT INTO estado (id_estado, descricao) VALUES (3, 'notsent');
INSERT INTO estado (id_estado, descricao) VALUES (4, 'declaration');

INSERT INTO concurso (
    referencia, entidade, dia_erro, hora_erro, dia_proposta, hora_proposta, preco, 
    tipo_id, plataforma_id, referencia_bc, preliminar, dia_audiencia, hora_audiencia, 
    final, recurso, impugnacao, estado_id
) 
VALUES (
    '123456789', 'Municipio Peniche', '2025-03-20', '10:00', '2025-03-22', '14:00', 10000, 
    1, 2, 'BC1234', TRUE, '2025-03-23', '09:00', FALSE, TRUE, FALSE, 1
);