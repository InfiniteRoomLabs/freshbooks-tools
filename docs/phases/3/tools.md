# Phase 3 tool surface (definitive, generated in stage 1 on 2026-09-01)

One tool per exported `freshbooks` service method, excluding the 17 `All` iterators (see `docs/phases/3/plan.md`, decision D1). Name = `{service_field_snake}_{method_snake}`; acronym runs collapse (`PDF` -> `pdf`, `CSV` -> `csv`, `FBPay` -> `fb_pay`). `keys` are the `// inventory:` keys the lib method carries; the tool registration carries the same keys, stacked. `identity_whoami` carries no key (it is a lib convenience over `GET /auth/api/v1/users/me`). The 213th inventory key, `Authorization/Revoke Refresh Token`, lives on `auth.Config.Revoke` and is NOT a tool (the MCP is a token consumer).

Annotation column: RO = `ReadOnlyHint: true`; D = `DestructiveHint: true` (delete/archive/remove); I = `IdempotentHint: true` (update/verify/undelete); W = plain write (create/send/register/upload/tokenize/invite/provision).

| # | tool | lib method | annot | keys |
|---|---|---|---|---|
| 1 | `attachments_upload_expense_receipt` | `Attachments.UploadExpenseReceipt` (attachments.go:35) | W | `Uploader/Upload Expense Receipt` |
| 2 | `bill_payments_create` | `BillPayments.Create` (bill_payments.go:78) | W | `Expenses/Bills (Beta)/Add Payment to Bill` |
| 3 | `bill_payments_update` | `BillPayments.Update` (bill_payments.go:100) | I | `Expenses/Bills (Beta)/Edit Payment to Bill` |
| 4 | `bill_vendors_list` | `BillVendors.List` (bill_vendors.go:156) | RO | `Expenses/Vendors (Beta)/Get Vendors` |
| 5 | `bill_vendors_create` | `BillVendors.Create` (bill_vendors.go:184) | W | `Expenses/Vendors (Beta)/Add Vendor` |
| 6 | `bill_vendors_update` | `BillVendors.Update` (bill_vendors.go:205) | I | `Expenses/Vendors (Beta)/Edit Vendor Details` |
| 7 | `bill_vendors_delete` | `BillVendors.Delete` (bill_vendors.go:227) | D | `Expenses/Vendors (Beta)/Delete Vendor` |
| 8 | `bills_list` | `Bills.List` (bills.go:154) | RO | `Expenses/Bills (Beta)/Get Bills` |
| 9 | `bills_create` | `Bills.Create` (bills.go:182) | W | `Expenses/Bills (Beta)/Add Bill from Vendor` |
| 10 | `bills_archive` | `Bills.Archive` (bills.go:222) | D | `Expenses/Bills (Beta)/Archive Bill` |
| 11 | `bills_delete` | `Bills.Delete` (bills.go:231) | D | `Expenses/Bills (Beta)/Delete Bill` |
| 12 | `callbacks_register` | `Callbacks.Register` (callbacks.go:83) | W | `Webhooks/Register for Callback` |
| 13 | `callbacks_list` | `Callbacks.List` (callbacks.go:118) | RO | `Webhooks/List Webhook Callbacks` |
| 14 | `callbacks_delete` | `Callbacks.Delete` (callbacks.go:151) | D | `Webhooks/Delete Webhook Callback` |
| 15 | `callbacks_verify` | `Callbacks.Verify` (callbacks.go:168) | I | `Webhooks/Verify Webhook Callback` |
| 16 | `callbacks_resend_verification` | `Callbacks.ResendVerification` (callbacks.go:198) | W | `Webhooks/Resend Verification Code` |
| 17 | `clients_list` | `Clients.List` (clients.go:177) | RO | `Clients/List Clients` |
| 18 | `clients_get` | `Clients.Get` (clients.go:205) | RO | `Clients/Single Client` |
| 19 | `clients_create` | `Clients.Create` (clients.go:220) | W | `Clients/New Client` |
| 20 | `clients_update` | `Clients.Update` (clients.go:241) | I | `Clients/Update Client` |
| 21 | `clients_remove_all_secondary_contacts` | `Clients.RemoveAllSecondaryContacts` (clients.go:264) | D | `Clients/Remove All Secondary Contacts` |
| 22 | `contacts_update` | `Contacts.Update` (contacts.go:58) | I | `Clients/Edit Secondary Contact ID` |
| 23 | `contacts_delete` | `Contacts.Delete` (contacts.go:81) | D | `Clients/Delete Secondary  Contact ID` |
| 24 | `credit_notes_list` | `CreditNotes.List` (credit_notes.go:150) | RO | `Clients/Credits/List Credits` |
| 25 | `credit_notes_create` | `CreditNotes.Create` (credit_notes.go:181) | W | `Clients/Credits/Create Credit Note`; `Clients/Credits/Create Prepayment Credit` |
| 26 | `credit_notes_update` | `CreditNotes.Update` (credit_notes.go:204) | I | `Clients/Credits/Update Credit Note`; `Clients/Credits/Update Prepayment Credit` |
| 27 | `credit_notes_delete` | `CreditNotes.Delete` (credit_notes.go:226) | D | `Clients/Credits/Delete Credit` |
| 28 | `estimates_list` | `Estimates.List` (estimates.go:201) | RO | `Estimates/List Estimates` |
| 29 | `estimates_get` | `Estimates.Get` (estimates.go:229) | RO | `Estimates/Single Estimate` |
| 30 | `estimates_create` | `Estimates.Create` (estimates.go:245) | W | `Estimates/Create Single Proposal w/ Sections, Logos, and E-signature`; `Estimates/Single Estimate With Estimate Lines` |
| 31 | `estimates_update` | `Estimates.Update` (estimates.go:266) | I | `Estimates/Update Estimate` |
| 32 | `estimates_delete` | `Estimates.Delete` (estimates.go:290) | D | `Estimates/Delete Estimate` |
| 33 | `estimates_accept` | `Estimates.Accept` (estimates.go:301) | I | `Estimates/Accept Estimate` |
| 34 | `estimates_send` | `Estimates.Send` (estimates.go:317) | W | `Estimates/Send Estimate by Email` |
| 35 | `expense_categories_list` | `ExpenseCategories.List` (expense_categories.go:93) | RO | `Expenses/List Expense Categories` |
| 36 | `expense_categories_get` | `ExpenseCategories.Get` (expense_categories.go:121) | RO | `Expenses/Single Expense Category` |
| 37 | `expense_categories_create` | `ExpenseCategories.Create` (expense_categories.go:138) | W | `Expenses/Create Custom Expense Category` |
| 38 | `expenses_list` | `Expenses.List` (expenses.go:171) | RO | `Expenses/List Expenses` |
| 39 | `expenses_get` | `Expenses.Get` (expenses.go:199) | RO | `Expenses/Single Expense` |
| 40 | `expenses_create` | `Expenses.Create` (expenses.go:218) | W | `Expenses/Create Expense`; `Expenses/Create Expense with Receipt` |
| 41 | `expenses_update` | `Expenses.Update` (expenses.go:241) | I | `Expenses/Update Expense`; `Expenses/Update Expense with Receipt` |
| 42 | `expenses_delete` | `Expenses.Delete` (expenses.go:270) | D | `Expenses/Delete Expense` |
| 43 | `expenses_summaries` | `Expenses.Summaries` (expenses.go:308) | RO | `Expenses/Expense Summaries` |
| 44 | `expenses_vendors` | `Expenses.Vendors` (expenses.go:334) | RO | `Expenses/Expense Vendors` |
| 45 | `expenses_create_recurring` | `Expenses.CreateRecurring` (expenses.go:404) | W | `Expenses/Create Recurring Expense` |
| 46 | `gateways_get` | `Gateways.Get` (gateways.go:101) | RO | `Tokenization/1a. [STRIPE] -  Get Publishable Key`; `Settings/Businesses/Gateway Details`; `Settings/Gateways/List Gateways` |
| 47 | `identity_me` | `Identity.Me` (identity.go:88) | RO | `Authorization/Identity Info Call`; `Authorization/List User` |
| 48 | `identity_whoami` | `Identity.Whoami` (identity.go:98) | RO | - |
| 49 | `identity_register` | `Identity.Register` (identity.go:135) | W | `Authorization/Register as a new user` |
| 50 | `images_upload` | `Images.Upload` (images.go:46) | W | `Uploader/Upload Logo or Proposal Image`; `Invoices/Upload Logo/Upload Logo`; `Expenses/Upload Expense Receipt Image/Upload Receipt Image` |
| 51 | `images_upload_without_account` | `Images.UploadWithoutAccount` (images.go:63) | W | `Uploader/Upload Image Without AccountId`; `Settings/Developer/Upload App Logo` |
| 52 | `invoice_profiles_list` | `InvoiceProfiles.List` (invoice_profiles.go:137) | RO | `Invoices/Invoice Recurring Template/List Invoice Profiles` |
| 53 | `invoice_profiles_get` | `InvoiceProfiles.Get` (invoice_profiles.go:164) | RO | `Invoices/Invoice Recurring Template/Get Single Invoice Profile` |
| 54 | `invoice_profiles_create` | `InvoiceProfiles.Create` (invoice_profiles.go:205) | W | `Invoices/Invoice Recurring Template/Create Single Invoice Profile`; `Invoices/Invoice Recurring Template/Create Single Invoice Profile w/ Time Entry Holder` |
| 55 | `invoice_profiles_update` | `InvoiceProfiles.Update` (invoice_profiles.go:232) | I | `Invoices/Invoice Recurring Template/Update Invoice Profile` |
| 56 | `invoice_profiles_delete` | `InvoiceProfiles.Delete` (invoice_profiles.go:250) | D | `Invoices/Invoice Recurring Template/Delete  Invoice Profile` |
| 57 | `invoice_profiles_enable_payment_options` | `InvoiceProfiles.EnablePaymentOptions` (invoice_profiles.go:264) | I | `Invoices/Invoice Recurring Template/Enable Payment Options On Invoice Profile` |
| 58 | `invoices_list` | `Invoices.List` (invoices.go:223) | RO | `Invoices/List Invoices` |
| 59 | `invoices_get` | `Invoices.Get` (invoices.go:253) | RO | `Invoices/Single Invoice`; `Invoices/Single Invoice w/ Logo` |
| 60 | `invoices_create` | `Invoices.Create` (invoices.go:288) | W | `Invoices/Create Invoice with Expense`; `Invoices/Single Invoice w/ Line Items`; `Invoices/Single Invoice w/ Logo and styles`; `Invoices/Single Invoice w/ Payment Gateway` |
| 61 | `invoices_update` | `Invoices.Update` (invoices.go:329) | I | `Invoices/Update Invoice`; `Invoices/Update Invoice w/ Expense`; `Invoices/Toggle Online Payments on Invoice` |
| 62 | `invoices_delete` | `Invoices.Delete` (invoices.go:348) | D | `Invoices/Delete  Invoice` |
| 63 | `invoices_send` | `Invoices.Send` (invoices.go:382) | W | `Invoices/Send Invoice by Email` |
| 64 | `invoices_pdf` | `Invoices.PDF` (invoices.go:411) | RO | `Invoices/Invoice Links/Downloads/Download Invoice PDF` |
| 65 | `invoices_share_link` | `Invoices.ShareLink` (invoices.go:454) | RO | `Invoices/Invoice Links/Downloads/Share Link`; `Invoices/Invoice Links/Downloads/Share PDF` |
| 66 | `invoices_enable_payment_options` | `Invoices.EnablePaymentOptions` (invoices.go:510) | I | `Invoices/Enable Payment Options On Invoice` |
| 67 | `invoices_invoice_presentation_defaults` | `Invoices.InvoicePresentationDefaults` (invoices.go:526) | RO | `Invoices/Get default invoice presentation styles` |
| 68 | `items_list` | `Items.List` (items.go:74) | RO | `Invoices/Items and Services/List Items`; `Invoices/Items and Services/List Items Filtered by SKU`; `Settings/Items and Services/List Items`; `Settings/Items and Services/List Items Filtered by SKU` |
| 69 | `items_get` | `Items.Get` (items.go:102) | RO | `Invoices/Items and Services/Single Item`; `Settings/Items and Services/Single Item` |
| 70 | `items_create` | `Items.Create` (items.go:128) | W | `Invoices/Items and Services/Create Item`; `Settings/Items and Services/Create Item` |
| 71 | `items_update` | `Items.Update` (items.go:158) | I | `Invoices/Items and Services/Update Item`; `Settings/Items and Services/Update Item` |
| 72 | `items_delete` | `Items.Delete` (items.go:177) | D | `Invoices/Items and Services/Delete Item`; `Settings/Items and Services/Delete Item` |
| 73 | `journal_entries_create` | `JournalEntries.Create` (journal_entries.go:141) | W | `Accounting/Journal Entries/Add Journal Entry` |
| 74 | `journal_entries_details` | `JournalEntries.Details` (journal_entries.go:164) | RO | `Accounting/Journal Entries/Journal Entry Details` |
| 75 | `journal_entry_accounts_list` | `JournalEntryAccounts.List` (journal_entries.go:212) | RO | `Accounting/Journal Entries/Accounts`; `Reports/General Ledger` |
| 76 | `ledger_accounts_create` | `LedgerAccounts.Create` (ledger_accounts.go:116) | W | `Accounting/Accounts/Create Account` |
| 77 | `ledger_accounts_list` | `LedgerAccounts.List` (ledger_accounts.go:140) | RO | `Accounting/Accounts/List Accounts` |
| 78 | `ledger_accounts_get` | `LedgerAccounts.Get` (ledger_accounts.go:157) | RO | `Accounting/Accounts/Single Account` |
| 79 | `ledger_accounts_update` | `LedgerAccounts.Update` (ledger_accounts.go:172) | I | `Accounting/Accounts/Update Account` |
| 80 | `ledger_accounts_types` | `LedgerAccounts.Types` (ledger_accounts.go:195) | RO | `Accounting/Accounts/List Account types` |
| 81 | `ledger_accounts_sub_types` | `LedgerAccounts.SubTypes` (ledger_accounts.go:211) | RO | `Accounting/Accounts/List Sub types` |
| 82 | `ledger_accounts_sub_type` | `LedgerAccounts.SubType` (ledger_accounts.go:226) | RO | `Accounting/Accounts/Single Sub type` |
| 83 | `other_income_create` | `OtherIncome.Create` (other_income.go:119) | W | `Accounting/Other Income/Create Single Other Income`; `Invoices/Other Income/Create Single Other Income` |
| 84 | `other_income_list` | `OtherIncome.List` (other_income.go:155) | RO | `Accounting/Other Income/List Other Income`; `Invoices/Other Income/List Other Income` |
| 85 | `other_income_update` | `OtherIncome.Update` (other_income.go:188) | I | `Accounting/Other Income/Update Single Other Income`; `Invoices/Other Income/Update Single Other Income` |
| 86 | `other_income_delete` | `OtherIncome.Delete` (other_income.go:215) | D | `Accounting/Other Income/Delete Single Other Income`; `Invoices/Other Income/Delete Single Other Income` |
| 87 | `payment_options_fb_pay_tokenize` | `PaymentOptions.FBPayTokenize` (payment_options.go:135) | W | `Tokenization/1. [FBPAY] - Create Payment Method` |
| 88 | `payment_options_stripe_tokenize` | `PaymentOptions.StripeTokenize` (payment_options.go:159) | W | `Tokenization/1. [STRIPE] - Create Payment Method` |
| 89 | `payment_options_stripe_create_setup_intent` | `PaymentOptions.StripeCreateSetupIntent` (payment_options.go:179) | W | `Tokenization/2. [STRIPE] - Create Setup Intent Using Payment Method Key` |
| 90 | `payment_options_save_credit_card` | `PaymentOptions.SaveCreditCard` (payment_options.go:200) | W | `Tokenization/2. [FBPAY] - Create Setup Intent Using Payment Method Key`; `Tokenization/3. [STRIPE] - Save Payment Method to Recurring Profile` |
| 91 | `payments_list` | `Payments.List` (payments.go:74) | RO | `Invoices/Payments/List Payments` |
| 92 | `payments_get` | `Payments.Get` (payments.go:101) | RO | `Invoices/Payments/Single Payment` |
| 93 | `payments_create` | `Payments.Create` (payments.go:125) | W | `Invoices/Payments/Make Payment` |
| 94 | `payments_update` | `Payments.Update` (payments.go:152) | I | `Invoices/Payments/Update Payment` |
| 95 | `payments_delete` | `Payments.Delete` (payments.go:170) | D | `Invoices/Payments/Delete Payment` |
| 96 | `payments_create_checkout_link` | `Payments.CreateCheckoutLink` (payments.go:251) | W | `Invoices/Payments/Single Checkout Link` |
| 97 | `payments_update_checkout_link` | `Payments.UpdateCheckoutLink` (payments.go:269) | I | `Invoices/Payments/Update Checkout Link` |
| 98 | `payments_delete_checkout_link` | `Payments.DeleteCheckoutLink` (payments.go:289) | D | `Invoices/Payments/Delete Checkout Link` |
| 99 | `payments_update_checkout_link_gateway` | `Payments.UpdateCheckoutLinkGateway` (payments.go:317) | I | `Invoices/Payments/Update Checkout Link Payment Gateway` |
| 100 | `projects_create` | `Projects.Create` (projects.go:81) | W | `Projects/Create Single Project` |
| 101 | `projects_get` | `Projects.Get` (projects.go:106) | RO | `Projects/Single Project` |
| 102 | `projects_list` | `Projects.List` (projects.go:140) | RO | `Projects/List Projects` |
| 103 | `projects_update` | `Projects.Update` (projects.go:182) | I | `Projects/Update Project` |
| 104 | `projects_delete` | `Projects.Delete` (projects.go:212) | D | `Projects/Delete Project` |
| 105 | `projects_abilities` | `Projects.Abilities` (projects.go:234) | RO | `Settings/Abilities/Abilities` |
| 106 | `projects_threads` | `Projects.Threads` (projects.go:248) | RO | `Projects/List All Messages in Project Discussion` |
| 107 | `projects_create_thread` | `Projects.CreateThread` (projects.go:262) | W | `Projects/Create New Message in Project Discussion` |
| 108 | `projects_add_thread_comment` | `Projects.AddThreadComment` (projects.go:277) | W | `Projects/Add Comment to Project Discussion Message` |
| 109 | `reports_accounts_aging` | `Reports.AccountsAging` (reports.go:111) | RO | `Reports/Accounts Aging` |
| 110 | `reports_balance_sheet` | `Reports.BalanceSheet` (reports.go:183) | RO | `Reports/Balance Sheet` |
| 111 | `reports_bank_reconciliation_summary` | `Reports.BankReconciliationSummary` (reports.go:219) | RO | `Reports/Bank Reconciliation Summary` |
| 112 | `reports_client_account_statement` | `Reports.ClientAccountStatement` (reports.go:254) | RO | `Reports/Client Account Statement` |
| 113 | `reports_download_invoice_details_csv` | `Reports.DownloadInvoiceDetailsCSV` (reports.go:273) | RO | `Reports/Download CSV Report` |
| 114 | `reports_expense_details` | `Reports.ExpenseDetails` (reports.go:316) | RO | `Reports/Expense Details` |
| 115 | `reports_invoice_details` | `Reports.InvoiceDetails` (reports.go:377) | RO | `Reports/Invoice Details` |
| 116 | `reports_item_sales` | `Reports.ItemSales` (reports.go:462) | RO | `Reports/Item Sales` |
| 117 | `reports_payments_collected` | `Reports.PaymentsCollected` (reports.go:514) | RO | `Reports/Payments Collected` |
| 118 | `reports_profit_loss` | `Reports.ProfitLoss` (reports.go:576) | RO | `Reports/Profit/Loss Report` |
| 119 | `reports_revenue_by_client` | `Reports.RevenueByClient` (reports.go:619) | RO | `Reports/Revenue By Client` |
| 120 | `reports_sales_tax_summary` | `Reports.SalesTaxSummary` (reports.go:679) | RO | `Reports/Sales Tax Summary` |
| 121 | `reports_trial_balance` | `Reports.TrialBalance` (reports.go:733) | RO | `Reports/Trial Balance` |
| 122 | `reports_time_entry_details` | `Reports.TimeEntryDetails` (reports.go:813) | RO | `Reports/Time Entry Details` |
| 123 | `retainers_list` | `Retainers.List` (retainers.go:76) | RO | `Invoices/Retainers/Get all retainers` |
| 124 | `retainers_get` | `Retainers.Get` (retainers.go:88) | RO | `Invoices/Retainers/Single Retainer` |
| 125 | `retainers_create` | `Retainers.Create` (retainers.go:118) | W | `Invoices/Retainers/Create Retainer` |
| 126 | `retainers_update` | `Retainers.Update` (retainers.go:149) | I | `Invoices/Retainers/Update Retainer` |
| 127 | `retainers_delete` | `Retainers.Delete` (retainers.go:165) | D | `Invoices/Retainers/Delete Retainer` |
| 128 | `retainers_undelete` | `Retainers.Undelete` (retainers.go:177) | I | `Invoices/Retainers/Undelete Retainer` |
| 129 | `service_rates_get` | `ServiceRates.Get` (service_rates.go:23) | RO | `Settings/Items and Services/Get a Single Service Rate` |
| 130 | `service_rates_list` | `ServiceRates.List` (service_rates.go:41) | RO | `Projects/Service Rates` |
| 131 | `service_rates_update_project_rate` | `ServiceRates.UpdateProjectRate` (service_rates.go:67) | I | `Projects/Update Service Rates` |
| 132 | `services_get` | `Services.Get` (services_svc.go:31) | RO | `Settings/Items and Services/Get a Single Service` |
| 133 | `services_list` | `Services.List` (services_svc.go:62) | RO | `Settings/Items and Services/List Services` |
| 134 | `services_create` | `Services.Create` (services_svc.go:125) | W | `Settings/Items and Services/Create Service` |
| 135 | `services_get_billable_item` | `Services.GetBillableItem` (services_svc.go:146) | RO | `Settings/Items and Services/Single Service` |
| 136 | `identity_add_business` | `Identity.AddBusiness` (settings.go:57) | W | `Settings/Businesses/Add Business` |
| 137 | `identity_delete_business` | `Identity.DeleteBusiness` (settings.go:75) | D | `Settings/Businesses/Delete Business` |
| 138 | `identity_delete_business_subscription` | `Identity.DeleteBusinessSubscription` (settings.go:93) | D | `Settings/Businesses/Delete Business - Subscription` |
| 139 | `identity_provision_payments` | `Identity.ProvisionPayments` (settings.go:124) | W | `Settings/Businesses/Provision FreshBooks Payments` |
| 140 | `identity_create_application` | `Identity.CreateApplication` (settings.go:180) | W | `Settings/Developer/Create new application` |
| 141 | `identity_applications` | `Identity.Applications` (settings.go:197) | RO | `Settings/Developer/Get all applications` |
| 142 | `identity_update_application` | `Identity.UpdateApplication` (settings.go:232) | I | `Settings/Developer/Modify existing application` |
| 143 | `staff_list` | `Staff.List` (staff.go:57) | RO | `My Team/List Staff` |
| 144 | `staff_get` | `Staff.Get` (staff.go:127) | RO | `My Team/Single Staff` |
| 145 | `staff_update` | `Staff.Update` (staff.go:151) | I | `My Team/Update Staff` |
| 146 | `staff_delete` | `Staff.Delete` (staff.go:172) | D | `My Team/Delete Staff` |
| 147 | `systems_get` | `Systems.Get` (systems.go:86) | RO | `Settings/Systems/Get System` |
| 148 | `tasks_create` | `Tasks.Create` (tasks.go:74) | W | `Projects/Tasks/Create Task` |
| 149 | `tasks_get` | `Tasks.Get` (tasks.go:93) | RO | `Projects/Tasks/Single Task` |
| 150 | `tasks_list` | `Tasks.List` (tasks.go:113) | RO | `Projects/Tasks/List Tasks` |
| 151 | `tasks_update` | `Tasks.Update` (tasks.go:156) | I | `Projects/Tasks/Update Task` |
| 152 | `tasks_delete` | `Tasks.Delete` (tasks.go:176) | D | `Projects/Tasks/Delete Task` |
| 153 | `taxes_list` | `Taxes.List` (taxes.go:101) | RO | `Expenses/List Taxes`; `Accounting/Taxes/List Taxes`; `Settings/Items and Services/List Taxes` |
| 154 | `taxes_get` | `Taxes.Get` (taxes.go:131) | RO | `Expenses/Single Tax (GET)`; `Accounting/Taxes/Get Single Tax`; `Settings/Items and Services/Single Tax (GET)` |
| 155 | `taxes_create` | `Taxes.Create` (taxes.go:148) | W | `Expenses/Create Single Tax`; `Accounting/Taxes/Create Single Tax`; `Settings/Items and Services/Create Single Tax` |
| 156 | `taxes_update` | `Taxes.Update` (taxes.go:171) | I | `Expenses/Update Tax`; `Accounting/Taxes/Update Single Tax`; `Settings/Items and Services/Update Tax` |
| 157 | `taxes_delete` | `Taxes.Delete` (taxes.go:196) | D | `Expenses/Single Tax (DELETE)`; `Accounting/Taxes/Delete Single Tax`; `Settings/Items and Services/Single Tax (DELETE)` |
| 158 | `team_members_list` | `TeamMembers.List` (team_members.go:98) | RO | `My Team/List Team Members` |
| 159 | `team_members_get` | `TeamMembers.Get` (team_members.go:129) | RO | `My Team/Single Team Member` |
| 160 | `team_members_invitation_rates` | `TeamMembers.InvitationRates` (team_members.go:157) | RO | `Projects/Invitation Rates` |
| 161 | `team_members_rates` | `TeamMembers.Rates` (team_members.go:180) | RO | `Projects/Team Member Rates` |
| 162 | `team_members_update_rate` | `TeamMembers.UpdateRate` (team_members.go:199) | I | `My Team/Update Staff Rates`; `Projects/Update Team Member Rate` |
| 163 | `team_members_invite` | `TeamMembers.Invite` (team_members.go:237) | W | `Projects/Invite Team Member to Project(s)` |
| 164 | `time_entries_list` | `TimeEntries.List` (time_entries.go:108) | RO | `Time Tracking/List Entries`; `Time Tracking/Time Entries Updated Since Precise Time`; `Time Tracking/Time Entries for a Given Day` |
| 165 | `time_entries_search` | `TimeEntries.Search` (time_entries.go:119) | RO | `Time Tracking/Time Entries For Employee on Specific Project` |
| 166 | `time_entries_create` | `TimeEntries.Create` (time_entries.go:144) | W | `Time Tracking/Create a Time Entry` |
| 167 | `time_entries_update` | `TimeEntries.Update` (time_entries.go:171) | I | `Time Tracking/Update a Time Entry` |
| 168 | `time_entries_delete` | `TimeEntries.Delete` (time_entries.go:187) | D | `Time Tracking/Delete a Time Entry` |
| 169 | `time_entries_list_with_totals` | `TimeEntries.ListWithTotals` (time_entries.go:164) | RO | - |

**Totals:** 169 tools, 212 inventory keys (+1 auth-owned key = 213). Row 169 (Phase 8 convergence, 2026-09-03) is keyless like `identity_whoami`: it wraps the same wire endpoint as row 164 (`time_entries_list`), not a distinct Postman request, so it carries none of that endpoint's three keys a second time.
