package authz

const (
	ResourceFinancialAccounting = "financial_accounting"
	ActionFinancialView         = "view"
)

var FinancialAccountingView = Permission{
	Resource: ResourceFinancialAccounting,
	Action:   ActionFinancialView,
}

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceFinancialAccounting,
		LabelKey: "Financial accounting",
		Actions: []ActionDefinition{
			{
				Action:         ActionFinancialView,
				LabelKey:       "View financial accounting",
				DescriptionKey: "View internal upstream costs and profit reporting.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})
}
