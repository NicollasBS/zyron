package auth

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// ADConfig armazena as configurações de conexão com o Active Directory.
type ADConfig struct {
	Host               string
	Port               string
	UseSSL             bool
	InsecureSkipVerify bool
	BaseDN             string
	BindUser           string
	BindPassword       string
	OperatorGroup      string
	ManagerGroup       string
}

// LoadADConfig carrega as configurações do Active Directory a partir das variáveis de ambiente.
func LoadADConfig() *ADConfig {
	host := os.Getenv("AD_HOST")
	port := os.Getenv("AD_PORT")
	if port == "" {
		port = "636" // Porta padrão LDAPS
	}

	useSSL := true
	if sslEnv := os.Getenv("AD_USE_SSL"); sslEnv != "" {
		useSSL = strings.ToLower(sslEnv) == "true" || sslEnv == "1"
	} else if port != "636" && port != "3269" {
		useSSL = false
	}

	skipVerify := strings.ToLower(os.Getenv("AD_INSECURE_SKIP_VERIFY")) == "true" || os.Getenv("AD_INSECURE_SKIP_VERIFY") == "1"

	operatorGroup := os.Getenv("AD_OPERATOR_GROUP")
	if operatorGroup == "" {
		operatorGroup = "Suporte_TI"
	}

	managerGroup := os.Getenv("AD_MANAGER_GROUP")
	if managerGroup == "" {
		managerGroup = "SG_GERENCIA_ATENDIMENTO"
	}

	return &ADConfig{
		Host:               host,
		Port:               port,
		UseSSL:             useSSL,
		InsecureSkipVerify: skipVerify,
		BaseDN:             os.Getenv("AD_BASE_DN"),
		BindUser:           os.Getenv("AD_BIND_USER"),
		BindPassword:       os.Getenv("AD_BIND_PASSWORD"),
		OperatorGroup:      operatorGroup,
		ManagerGroup:       managerGroup,
	}
}

// UserProfile representa as informações de perfil e identificação do usuário obtidas do Active Directory.
type UserProfile struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	IsOperator  bool   `json:"is_operator"`
	IsManager   bool   `json:"is_manager"`
}

// ADService define a interface para validação de perfis no Active Directory.
type ADService interface {
	CheckIsOperator(username string) (bool, error)
	CheckIsManager(username string) (bool, error)
	GetUserProfile(username string) (*UserProfile, error)
	AuthenticateUser(username, password string) (*UserProfile, error)
}

// LDAPService implementa a conexão real com o servidor Active Directory via LDAP/LDAPS.
type LDAPService struct {
	Config *ADConfig
}

// NewLDAPService cria uma nova instância de serviço LDAP.
func NewLDAPService(cfg *ADConfig) *LDAPService {
	if cfg == nil {
		cfg = LoadADConfig()
	}
	return &LDAPService{Config: cfg}
}

// CheckIsOperator valida se o usuário informado pertence ao grupo de operadores no Active Directory.
func (s *LDAPService) CheckIsOperator(username string) (bool, error) {
	profile, err := s.GetUserProfile(username)
	if err != nil {
		return false, err
	}
	return profile.IsOperator, nil
}

// CheckIsManager valida se o usuário informado pertence ao grupo de gerência (SG_GERENCIA_ATENDIMENTO).
func (s *LDAPService) CheckIsManager(username string) (bool, error) {
	profile, err := s.GetUserProfile(username)
	if err != nil {
		return false, err
	}
	return profile.IsManager, nil
}

// GetUserProfile consulta os dados completos do usuário no Active Directory (is_operator, is_manager e display_name).
func (s *LDAPService) GetUserProfile(username string) (*UserProfile, error) {
	cleanAccount := extractAccountName(username)
	lower := strings.ToLower(username)

	// Fallback para mock caso o servidor de AD não tenha sido configurado no ambiente
	if s.Config.Host == "" {
		log.Printf("[AD] AD_HOST não configurado. Utilizando fallback local para '%s'", username)
		isOp := strings.Contains(lower, "suporte") || strings.Contains(lower, "admin")
		isManager := strings.Contains(lower, "gerencia") || strings.Contains(lower, "gestor") || strings.Contains(lower, "admin") || strings.Contains(lower, "185033")
		displayName := cleanAccount
		parts := strings.Split(cleanAccount, ".")
		if len(parts) > 1 {
			var capitalized []string
			for _, p := range parts {
				if len(p) > 0 {
					capitalized = append(capitalized, strings.ToUpper(p[:1])+p[1:])
				}
			}
			displayName = strings.Join(capitalized, " ")
		}
		return &UserProfile{
			Username:    username,
			DisplayName: displayName,
			IsOperator:  isOp,
			IsManager:   isManager,
		}, nil
	}

	addr := fmt.Sprintf("%s:%s", s.Config.Host, s.Config.Port)
	var l *ldap.Conn
	var err error

	if s.Config.UseSSL {
		tlsConfig := &tls.Config{
			ServerName:         s.Config.Host,
			InsecureSkipVerify: s.Config.InsecureSkipVerify,
		}
		l, err = ldap.DialTLS("tcp", addr, tlsConfig)
	} else {
		l, err = ldap.Dial("tcp", addr)
	}

	if err != nil {
		return nil, fmt.Errorf("falha ao conectar no Active Directory em %s: %w", addr, err)
	}
	defer l.Close()

	// Autenticação da conta de serviço para consulta no catálogo
	if s.Config.BindUser != "" {
		if err := l.Bind(s.Config.BindUser, s.Config.BindPassword); err != nil {
			return nil, fmt.Errorf("falha no bind da conta de serviço AD (%s): %w", s.Config.BindUser, err)
		}
	}

	// Monta filtro LDAP seguro contra injeção
	escapedClean := ldap.EscapeFilter(cleanAccount)
	escapedRaw := ldap.EscapeFilter(username)
	filter := fmt.Sprintf("(&(objectClass=user)(|(sAMAccountName=%s)(userPrincipalName=%s)))", escapedClean, escapedRaw)

	searchReq := ldap.NewSearchRequest(
		s.Config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 10, false,
		filter,
		[]string{"dn", "sAMAccountName", "userPrincipalName", "memberOf", "displayName", "name", "cn"},
		nil,
	)

	sr, err := l.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar usuário '%s' no AD: %w", username, err)
	}

	if len(sr.Entries) == 0 {
		log.Printf("[AD] Usuário '%s' não encontrado no diretório (%s)", username, s.Config.BaseDN)
		return &UserProfile{
			Username:    username,
			DisplayName: cleanAccount,
			IsOperator:  false,
			IsManager:   false,
		}, nil
	}

	userEntry := sr.Entries[0]
	displayName := userEntry.GetAttributeValue("displayName")
	if displayName == "" {
		displayName = userEntry.GetAttributeValue("name")
	}
	if displayName == "" {
		displayName = userEntry.GetAttributeValue("cn")
	}
	if displayName == "" {
		displayName = cleanAccount
	}

	memberOf := userEntry.GetAttributeValues("memberOf")
	targetGroup := strings.ToLower(s.Config.OperatorGroup)
	targetManagerGroup := strings.ToLower(s.Config.ManagerGroup)
	isOperator := false
	isManager := false

	// Verifica se o usuário pertence aos grupos configurados
	for _, groupDN := range memberOf {
		lowerGroupDN := strings.ToLower(groupDN)
		if strings.Contains(lowerGroupDN, "cn="+targetGroup+",") ||
			strings.HasSuffix(lowerGroupDN, "cn="+targetGroup) ||
			lowerGroupDN == targetGroup {
			isOperator = true
		}
		if strings.Contains(lowerGroupDN, "cn="+targetManagerGroup+",") ||
			strings.HasSuffix(lowerGroupDN, "cn="+targetManagerGroup) ||
			lowerGroupDN == targetManagerGroup {
			isManager = true
		}
	}

	log.Printf("[AD] Usuário '%s' resolvido: Nome='%s', Operador=%v, Gerente=%v", username, displayName, isOperator, isManager)
	return &UserProfile{
		Username:    username,
		DisplayName: displayName,
		IsOperator:  isOperator,
		IsManager:   isManager,
	}, nil
}

// AuthenticateUser autentica as credenciais do usuário e valida se possui acesso de gerência.
func (s *LDAPService) AuthenticateUser(username, password string) (*UserProfile, error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return nil, fmt.Errorf("usuário e senha são obrigatórios")
	}

	cleanAccount := extractAccountName(username)

	// Fallback mock quando AD_HOST não estiver configurado
	if s.Config.Host == "" {
		profile, err := s.GetUserProfile(username)
		if err != nil {
			return nil, err
		}
		if !profile.IsManager {
			return nil, fmt.Errorf("acesso negado: usuário '%s' não pertence ao grupo %s", username, s.Config.ManagerGroup)
		}
		return profile, nil
	}

	addr := fmt.Sprintf("%s:%s", s.Config.Host, s.Config.Port)
	var l *ldap.Conn
	var err error

	if s.Config.UseSSL {
		tlsConfig := &tls.Config{
			ServerName:         s.Config.Host,
			InsecureSkipVerify: s.Config.InsecureSkipVerify,
		}
		l, err = ldap.DialTLS("tcp", addr, tlsConfig)
	} else {
		l, err = ldap.Dial("tcp", addr)
	}

	if err != nil {
		return nil, fmt.Errorf("falha ao conectar no Active Directory: %w", err)
	}
	defer l.Close()

	// 1. Bind com conta de serviço para localizar o DN do usuário
	if s.Config.BindUser != "" {
		if err := l.Bind(s.Config.BindUser, s.Config.BindPassword); err != nil {
			return nil, fmt.Errorf("falha no bind de serviço AD: %w", err)
		}
	}

	escapedClean := ldap.EscapeFilter(cleanAccount)
	escapedRaw := ldap.EscapeFilter(username)
	filter := fmt.Sprintf("(&(objectClass=user)(|(sAMAccountName=%s)(userPrincipalName=%s)))", escapedClean, escapedRaw)

	searchReq := ldap.NewSearchRequest(
		s.Config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 10, false,
		filter,
		[]string{"dn", "sAMAccountName", "userPrincipalName", "memberOf", "displayName", "name", "cn"},
		nil,
	)

	sr, err := l.Search(searchReq)
	if err != nil || len(sr.Entries) == 0 {
		return nil, fmt.Errorf("usuário '%s' não encontrado no Active Directory", username)
	}

	userEntry := sr.Entries[0]
	userDN := userEntry.DN

	// 2. Valida a senha do usuário fazendo bind com seu próprio DN
	var userConn *ldap.Conn
	if s.Config.UseSSL {
		userConn, err = ldap.DialTLS("tcp", addr, &tls.Config{
			ServerName:         s.Config.Host,
			InsecureSkipVerify: s.Config.InsecureSkipVerify,
		})
	} else {
		userConn, err = ldap.Dial("tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir conexão para validação de credenciais: %w", err)
	}
	defer userConn.Close()

	if err := userConn.Bind(userDN, password); err != nil {
		return nil, fmt.Errorf("credenciais inválidas")
	}

	// 3. Verifica se possui o grupo de gerência
	memberOf := userEntry.GetAttributeValues("memberOf")
	targetManagerGroup := strings.ToLower(s.Config.ManagerGroup)
	isManager := false
	for _, groupDN := range memberOf {
		lowerGroupDN := strings.ToLower(groupDN)
		if strings.Contains(lowerGroupDN, "cn="+targetManagerGroup+",") ||
			strings.HasSuffix(lowerGroupDN, "cn="+targetManagerGroup) ||
			lowerGroupDN == targetManagerGroup {
			isManager = true
			break
		}
	}

	if !isManager {
		return nil, fmt.Errorf("acesso negado: usuário '%s' não pertence ao grupo %s no Active Directory", username, s.Config.ManagerGroup)
	}

	displayName := userEntry.GetAttributeValue("displayName")
	if displayName == "" {
		displayName = userEntry.GetAttributeValue("name")
	}
	if displayName == "" {
		displayName = cleanAccount
	}

	return &UserProfile{
		Username:    username,
		DisplayName: displayName,
		IsManager:   true,
	}, nil
}

// extractAccountName extrai o sAMAccountName a partir de DOMINIO\usuario ou usuario@dominio.
func extractAccountName(username string) string {
	if idx := strings.LastIndex(username, `\`); idx != -1 {
		return username[idx+1:]
	}
	if idx := strings.Index(username, "@"); idx != -1 {
		return username[:idx]
	}
	return username
}
