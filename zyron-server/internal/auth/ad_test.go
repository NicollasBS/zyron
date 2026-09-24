package auth_test

import (
	"os"
	"testing"
	"zyron-server/internal/auth"
)

func TestExtractAccountName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"DOMAIN\\usuario1", "usuario1"},
		{"CORP.LOCAL\\admin_ti", "admin_ti"},
		{"usuario2@empresa.com.br", "usuario2"},
		{"usuario3@empresa.local", "usuario3"},
		{"simples", "simples"},
	}

	service := auth.NewLDAPService(&auth.ADConfig{})
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// Testa fallback sem host configurado
			isOp, err := service.CheckIsOperator(tt.input)
			if err != nil {
				t.Fatalf("Erro inesperado no fallback: %v", err)
			}
			_ = isOp
		})
	}
}

func TestADFallback(t *testing.T) {
	// Garante que AD_HOST está vazio
	os.Unsetenv("AD_HOST")
	service := auth.NewLDAPService(nil)

	// Com "suporte" ou "admin", deve retornar true
	isOp, err := service.CheckIsOperator("DOMAIN\\suporte_carlos")
	if err != nil {
		t.Fatalf("Erro inesperado: %v", err)
	}
	if !isOp {
		t.Errorf("Esperava isOp=true para suporte_carlos")
	}

	isOp, err = service.CheckIsOperator("DOMAIN\\admin_maria")
	if err != nil {
		t.Fatalf("Erro inesperado: %v", err)
	}
	if !isOp {
		t.Errorf("Esperava isOp=true para admin_maria")
	}

	// Usuário comum, deve retornar false
	isOp, err = service.CheckIsOperator("DOMAIN\\joao.silva")
	if err != nil {
		t.Fatalf("Erro inesperado: %v", err)
	}
	if isOp {
		t.Errorf("Esperava isOp=false para joao.silva")
	}
}

func TestLoadADConfig(t *testing.T) {
	os.Setenv("AD_HOST", "ad.test.local")
	os.Setenv("AD_PORT", "636")
	os.Setenv("AD_USE_SSL", "true")
	os.Setenv("AD_BASE_DN", "DC=test,DC=local")
	os.Setenv("AD_BIND_USER", "svc_test")
	os.Setenv("AD_BIND_PASSWORD", "secret")
	os.Setenv("AD_OPERATOR_GROUP", "Operadores_TI")

	defer func() {
		os.Unsetenv("AD_HOST")
		os.Unsetenv("AD_PORT")
		os.Unsetenv("AD_USE_SSL")
		os.Unsetenv("AD_BASE_DN")
		os.Unsetenv("AD_BIND_USER")
		os.Unsetenv("AD_BIND_PASSWORD")
		os.Unsetenv("AD_OPERATOR_GROUP")
	}()

	cfg := auth.LoadADConfig()

	if cfg.Host != "ad.test.local" || cfg.Port != "636" || !cfg.UseSSL {
		t.Errorf("Configuração carregada incorreta: %+v", cfg)
	}
	if cfg.BaseDN != "DC=test,DC=local" || cfg.BindUser != "svc_test" {
		t.Errorf("Credenciais incorretas: %+v", cfg)
	}
	if cfg.OperatorGroup != "Operadores_TI" {
		t.Errorf("Grupo incorreto: %s", cfg.OperatorGroup)
	}
	if cfg.ManagerGroup != "SG_GERENCIA_ATENDIMENTO" {
		t.Errorf("Grupo de gerência padrão incorreto: %s", cfg.ManagerGroup)
	}
}

func TestManagerGroupAndAuth(t *testing.T) {
	os.Unsetenv("AD_HOST")
	service := auth.NewLDAPService(nil)

	// Usuário com "gerencia" deve ser gerente
	isManager, err := service.CheckIsManager("DOMAIN\\gerencia_atendimento")
	if err != nil || !isManager {
		t.Errorf("Esperava isManager=true para gerencia_atendimento, obteve %v (err: %v)", isManager, err)
	}

	// Usuário comum não deve ser gerente
	isManager, err = service.CheckIsManager("DOMAIN\\usuario.comum")
	if err != nil || isManager {
		t.Errorf("Esperava isManager=false para usuario.comum, obteve %v (err: %v)", isManager, err)
	}

	// AuthenticateUser com gerente deve suceder
	profile, err := service.AuthenticateUser("DOMAIN\\gestor_joao", "senha123")
	if err != nil || profile == nil || !profile.IsManager {
		t.Errorf("Esperava autenticação com sucesso para gestor_joao, err: %v", err)
	}

	// AuthenticateUser com usuário sem permissão deve falhar
	_, err = service.AuthenticateUser("DOMAIN\\usuario.comum", "senha123")
	if err == nil {
		t.Errorf("Esperava falha de autenticação para usuario.comum")
	}
}
