// story: e05s01
package budget

// defaultGroups is the starter set every new user gets (pt-BR), in display
// order. Edit freely: only users created after the change get the new list.
var defaultGroups = []struct {
	kind       string // "expense" or "income"
	name       string
	categories []string
}{
	{"expense", "Moradia", []string{"Aluguel ou financiamento", "Condomínio", "Energia", "Água", "Gás", "Internet", "Manutenção da casa"}},
	{"expense", "Alimentação", []string{"Mercado", "Restaurantes e delivery"}},
	{"expense", "Transporte", []string{"Combustível", "Transporte público", "Aplicativos de transporte", "Manutenção do veículo", "IPVA e licenciamento", "Estacionamento"}},
	{"expense", "Saúde", []string{"Plano de saúde", "Farmácia", "Consultas e exames"}},
	{"expense", "Educação", []string{"Mensalidades", "Cursos e livros"}},
	{"expense", "Contas e assinaturas", []string{"Celular", "Streaming", "Outras assinaturas"}},
	{"expense", "Pessoal", []string{"Vestuário", "Cuidados pessoais", "Presentes"}},
	{"expense", "Lazer", []string{"Passeios", "Viagens", "Hobbies"}},
	{"expense", "Financeiro", []string{"Impostos", "Tarifas bancárias", "Juros e multas", "Investimentos"}},
	{"income", "Receitas", []string{"Salário", "Renda extra", "Rendimentos", "Outras receitas"}},
}
