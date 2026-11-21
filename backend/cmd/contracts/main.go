package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/internal/modules/agreement"
	"baituber.cn/yanxuan/v2/internal/modules/audit"
	"baituber.cn/yanxuan/v2/internal/modules/content"
	"baituber.cn/yanxuan/v2/internal/modules/enrollment"
	"baituber.cn/yanxuan/v2/internal/modules/feedback"
	"baituber.cn/yanxuan/v2/internal/modules/files"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/notification"
	"baituber.cn/yanxuan/v2/internal/modules/settlement"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/modules/wallet"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

type enumValue struct {
	Name  string
	Value string
}

func main() {
	definitions := readEnums()
	writeEnums(definitions)
	writeModels()
}

func readEnums() map[string][]enumValue {
	definitions := make(map[string][]enumValue)
	paths, err := filepath.Glob("internal/domain/enum/*.go")
	if err != nil {
		panic(err)
	}

	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			panic(err)
		}

		for _, declaration := range file.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok || group.Tok != token.CONST {
				continue
			}

			for _, specification := range group.Specs {
				value := specification.(*ast.ValueSpec)
				enumType, ok := value.Type.(*ast.Ident)
				if !ok {
					continue
				}

				literal := value.Values[0].(*ast.BasicLit)
				wireValue, err := strconv.Unquote(literal.Value)
				if err != nil {
					panic(err)
				}

				definitions[enumType.Name] = append(definitions[enumType.Name], enumValue{

					Name:  value.Names[0].Name,
					Value: wireValue,
				})
			}
		}
	}

	return definitions
}

func writeEnums(definitions map[string][]enumValue) {
	var typescript, javascript bytes.Buffer
	typescript.WriteString("// Code generated from Go enum definitions; DO NOT EDIT.\n\n")
	javascript.WriteString("// Code generated from Go enum definitions; DO NOT EDIT.\n\n")
	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}

	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&typescript, "export enum %s {\n", name)
		fmt.Fprintf(&javascript, "const %s = Object.freeze({\n", name)
		for _, value := range definitions[name] {
			fmt.Fprintf(&typescript, "  %s = %q,\n", value.Name, value.Value)
			fmt.Fprintf(&javascript, "  %s: %q,\n", value.Name, value.Value)
		}

		typescript.WriteString("}\n\n")
		javascript.WriteString("});\n\n")
	}

	javascript.WriteString("module.exports = Object.freeze({\n")
	for _, name := range names {
		fmt.Fprintf(&javascript, "  %s,\n", name)
	}

	javascript.WriteString("});\n")

	write("../apps/admin/src/generated/enums.ts", typescript.Bytes())
	write("../apps/miniapp/src/services/generated/enums.js", javascript.Bytes())
}

func writeModels() {
	models := map[string]reflect.Type{
		"LoginResult":               reflect.TypeOf(identity.LoginResult{}),
		"Operator":                  reflect.TypeOf(identity.OperatorDTO{}),
		"OperatorPage":              reflect.TypeOf(identity.OperatorPage{}),
		"OperatorEndpoint":          reflect.TypeOf(identity.OperatorEndpoint{}),
		"OperatorPermissionCatalog": reflect.TypeOf(identity.OperatorPermissionCatalog{}),
		"User":                      reflect.TypeOf(identity.UserDTO{}),
		"DistributorBlock":          reflect.TypeOf(identity.DistributorBlockDTO{}),
		"DistributorDetails":        reflect.TypeOf(identity.DistributorDetailsDTO{}),
		"DistributorTask":           reflect.TypeOf(task.DistributorTaskDTO{}),
		"VerifiedIdentity":          reflect.TypeOf(identity.VerifiedIdentityDTO{}),
		"TaskInput":                 reflect.TypeOf(task.TaskInput{}),
		"TaskVersion":               reflect.TypeOf(task.VersionDTO{}),
		"MarketingSettings":         reflect.TypeOf(task.MarketingSettingsDTO{}),
		"MarketingSettingsInput":    reflect.TypeOf(task.MarketingSettingsInput{}),
		"MarketingPriceChange":      reflect.TypeOf(task.MarketingPriceChange{}),
		"PoolAllocation":            reflect.TypeOf(task.PoolAllocationDTO{}),
		"PoolExpiry":                reflect.TypeOf(task.PoolExpiryDTO{}),
		"Assignment":                reflect.TypeOf(task.AssignmentDTO{}),
		"Submission":                reflect.TypeOf(task.SubmissionDTO{}),
		"Step":                      reflect.TypeOf(task.StepDTO{}),
		"Wallet":                    reflect.TypeOf(wallet.WalletDTO{}),
		"LedgerEntry":               reflect.TypeOf(wallet.LedgerDTO{}),
		"Withdrawal":                reflect.TypeOf(wallet.WithdrawalDTO{}),
		"SettlementBatch":           reflect.TypeOf(settlement.BatchDTO{}),
		"SettlementPreview":         reflect.TypeOf(settlement.SettlementPreview{}),
		"SettlementLine":            reflect.TypeOf(settlement.SettlementPreviewLine{}),
		"Content":                   reflect.TypeOf(content.DTO{}),
		"Agreement":                 reflect.TypeOf(agreement.DTO{}),
		"Feedback":                  reflect.TypeOf(feedback.DTO{}),
		"AuditEvent":                reflect.TypeOf(audit.DTO{}),
		"IdentityRecord":            reflect.TypeOf(audit.IdentityRecordDTO{}),
		"IdentityRecordDetails":     reflect.TypeOf(audit.IdentityRecordDetailsDTO{}),
		"IdentityRecords":           reflect.TypeOf(audit.IdentityRecordsDTO{}),
		"EnrollmentDocuments":       reflect.TypeOf(audit.EnrollmentDocumentsDTO{}),
		"EnrollmentPhotoPreview":    reflect.TypeOf(identity.EnrollmentPhotoPreviewDTO{}),
		"MarketingTag":              reflect.TypeOf(enrollment.MarketingTagDTO{}),
		"Notification":              reflect.TypeOf(notification.DTO{}),
		"FileObject":                reflect.TypeOf(files.DTO{}),
		"FileInitiateInput":         reflect.TypeOf(files.InitiateInput{}),
		"FileUploadTicket":          reflect.TypeOf(files.UploadTicket{}),
		"FileUploadFields":          reflect.TypeOf(objectstore.UploadFields{}),
		"FileDownload":              reflect.TypeOf(files.DownloadDTO{}),
	}

	reverse := make(map[reflect.Type]string, len(models))
	names := make([]string, 0, len(models))
	for name, model := range models {
		reverse[model] = name
		names = append(names, name)
	}

	sort.Strings(names)
	var output bytes.Buffer
	output.WriteString("// Code generated from Go DTOs; DO NOT EDIT.\n\n")
	output.WriteString("import type * as Enums from './enums';\n\n")
	for _, name := range names {
		fmt.Fprintf(&output, "export interface %s {\n", name)
		writeFields(&output, models[name], reverse)
		output.WriteString("}\n\n")
	}

	write("../apps/admin/src/generated/models.ts", output.Bytes())
}

func writeFields(output *bytes.Buffer, model reflect.Type, names map[reflect.Type]string) {
	for index := 0; index < model.NumField(); index++ {
		field := model.Field(index)
		if field.Anonymous {
			writeFields(output, field.Type, names)

			continue
		}

		jsonTag := strings.Split(field.Tag.Get("json"), ",")
		name := jsonTag[0]
		if name == "" || name == "-" {
			continue
		}

		optional := ""
		for _, option := range jsonTag[1:] {
			if option == "omitempty" {
				optional = "?"
			}
		}

		fmt.Fprintf(output, "  %q%s: %s;\n", name, optional, typeScriptType(field.Type, names))
	}
}

func typeScriptType(value reflect.Type, names map[reflect.Type]string) string {
	if value == reflect.TypeOf(time.Time{}) {
		return "string"
	}

	if value.Kind() == reflect.Pointer {
		return typeScriptType(value.Elem(), names) + " | null"
	}

	if strings.HasSuffix(value.PkgPath(), "/domain/enum") {
		return "Enums." + value.Name()
	}

	if name, exists := names[value]; exists {
		return name
	}

	switch value.Kind() {
	case reflect.String, reflect.Uint, reflect.Uint64:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int32, reflect.Int64:
		return "number"
	case reflect.Slice:
		return typeScriptType(value.Elem(), names) + "[]"
	default:
		panic("unsupported DTO type: " + value.String())
	}
}

func write(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		panic(err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		panic(err)
	}
}
