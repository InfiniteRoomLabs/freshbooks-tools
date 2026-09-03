# Phase 4 command surface (definitive, generated in stage 1 on 2026-09-01)

One cobra command per exported `freshbooks` service method, derived row-for-row from `docs/phases/3/tools.md` (same 168 lib methods, same inventory keys, same read-only/destructive/idempotent classes). Group = the service field in kebab-case; verb = the method in kebab-case (`PDF` -> `pdf`, `CSV` -> `csv`, `FBPay` -> `fb-pay`). List commands whose service has an `All` iterator take `--all` (17 services), which is how the iterators are covered; no separate command exists for them. Non-registry commands: `auth login|status|logout|token`, `config view|contexts|use-context|set-context`, `identity me` is a registry row, `api <METHOD> <path>`, `completion`, `version`.

Flags column: S = scope (`--account` or `--business` or `--business-uuid` from the registry's scope family, default from the current context); ID = positional `<id>`; L = `--page/--per-page/--search k=v...` (+`--include` where the lib option has it, `--all` where the service has an iterator); B = `-f file|-` JSON body; other flags named explicitly.

| # | command | lib method | annot | flags | inventory keys |
|---|---|---|---|---|---|
| 1 | `freshbooks attachments upload-expense-receipt` | `Attachments.UploadExpenseReceipt` | W | S, `--file <path>` | `Uploader/Upload Expense Receipt` |
| 2 | `freshbooks bill-payments create` | `BillPayments.Create` | W | S, B | `Expenses/Bills (Beta)/Add Payment to Bill` |
| 3 | `freshbooks bill-payments update` | `BillPayments.Update` | I | S, ID, B | `Expenses/Bills (Beta)/Edit Payment to Bill` |
| 4 | `freshbooks bill-vendors list` | `BillVendors.List` | RO | S, L+all | `Expenses/Vendors (Beta)/Get Vendors` |
| 5 | `freshbooks bill-vendors create` | `BillVendors.Create` | W | S, B | `Expenses/Vendors (Beta)/Add Vendor` |
| 6 | `freshbooks bill-vendors update` | `BillVendors.Update` | I | S, ID, B | `Expenses/Vendors (Beta)/Edit Vendor Details` |
| 7 | `freshbooks bill-vendors delete` | `BillVendors.Delete` | D | S, ID | `Expenses/Vendors (Beta)/Delete Vendor` |
| 8 | `freshbooks bills list` | `Bills.List` | RO | S, L+all | `Expenses/Bills (Beta)/Get Bills` |
| 9 | `freshbooks bills create` | `Bills.Create` | W | S, B | `Expenses/Bills (Beta)/Add Bill from Vendor` |
| 10 | `freshbooks bills archive` | `Bills.Archive` | D | S, ID | `Expenses/Bills (Beta)/Archive Bill` |
| 11 | `freshbooks bills delete` | `Bills.Delete` | D | S, ID | `Expenses/Bills (Beta)/Delete Bill` |
| 12 | `freshbooks callbacks register` | `Callbacks.Register` | W | S, B | `Webhooks/Register for Callback` |
| 13 | `freshbooks callbacks list` | `Callbacks.List` | RO | S, L+all | `Webhooks/List Webhook Callbacks` |
| 14 | `freshbooks callbacks delete` | `Callbacks.Delete` | D | S, ID | `Webhooks/Delete Webhook Callback` |
| 15 | `freshbooks callbacks verify` | `Callbacks.Verify` | I | S, ID, `--verifier` | `Webhooks/Verify Webhook Callback` |
| 16 | `freshbooks callbacks resend-verification` | `Callbacks.ResendVerification` | W | S, ID | `Webhooks/Resend Verification Code` |
| 17 | `freshbooks clients list` | `Clients.List` | RO | S, L+all | `Clients/List Clients` |
| 18 | `freshbooks clients get` | `Clients.Get` | RO | S, ID | `Clients/Single Client` |
| 19 | `freshbooks clients create` | `Clients.Create` | W | S, B | `Clients/New Client` |
| 20 | `freshbooks clients update` | `Clients.Update` | I | S, ID, B | `Clients/Update Client` |
| 21 | `freshbooks clients remove-all-secondary-contacts` | `Clients.RemoveAllSecondaryContacts` | D | S, ID | `Clients/Remove All Secondary Contacts` |
| 22 | `freshbooks contacts update` | `Contacts.Update` | I | S, ID, B | `Clients/Edit Secondary Contact ID` |
| 23 | `freshbooks contacts delete` | `Contacts.Delete` | D | S, ID | `Clients/Delete Secondary  Contact ID` |
| 24 | `freshbooks credit-notes list` | `CreditNotes.List` | RO | S, L+all | `Clients/Credits/List Credits` |
| 25 | `freshbooks credit-notes create` | `CreditNotes.Create` | W | S, B | `Clients/Credits/Create Credit Note`; `Clients/Credits/Create Prepayment Credit` |
| 26 | `freshbooks credit-notes update` | `CreditNotes.Update` | I | S, ID, B | `Clients/Credits/Update Credit Note`; `Clients/Credits/Update Prepayment Credit` |
| 27 | `freshbooks credit-notes delete` | `CreditNotes.Delete` | D | S, ID | `Clients/Credits/Delete Credit` |
| 28 | `freshbooks estimates list` | `Estimates.List` | RO | S, L+all | `Estimates/List Estimates` |
| 29 | `freshbooks estimates get` | `Estimates.Get` | RO | S, ID | `Estimates/Single Estimate` |
| 30 | `freshbooks estimates create` | `Estimates.Create` | W | S, B | `Estimates/Create Single Proposal w/ Sections, Logos, and E-signature`; `Estimates/Single Estimate With Estimate Lines` |
| 31 | `freshbooks estimates update` | `Estimates.Update` | I | S, ID, B | `Estimates/Update Estimate` |
| 32 | `freshbooks estimates delete` | `Estimates.Delete` | D | S, ID | `Estimates/Delete Estimate` |
| 33 | `freshbooks estimates accept` | `Estimates.Accept` | I | S, ID | `Estimates/Accept Estimate` |
| 34 | `freshbooks estimates send` | `Estimates.Send` | W | S, ID, B | `Estimates/Send Estimate by Email` |
| 35 | `freshbooks expense-categories list` | `ExpenseCategories.List` | RO | S, L+all | `Expenses/List Expense Categories` |
| 36 | `freshbooks expense-categories get` | `ExpenseCategories.Get` | RO | S, ID | `Expenses/Single Expense Category` |
| 37 | `freshbooks expense-categories create` | `ExpenseCategories.Create` | W | S, B | `Expenses/Create Custom Expense Category` |
| 38 | `freshbooks expenses list` | `Expenses.List` | RO | S, L+all | `Expenses/List Expenses` |
| 39 | `freshbooks expenses get` | `Expenses.Get` | RO | S, ID | `Expenses/Single Expense` |
| 40 | `freshbooks expenses create` | `Expenses.Create` | W | S, B | `Expenses/Create Expense`; `Expenses/Create Expense with Receipt` |
| 41 | `freshbooks expenses update` | `Expenses.Update` | I | S, ID, B | `Expenses/Update Expense`; `Expenses/Update Expense with Receipt` |
| 42 | `freshbooks expenses delete` | `Expenses.Delete` | D | S, ID | `Expenses/Delete Expense` |
| 43 | `freshbooks expenses summaries` | `Expenses.Summaries` | RO | S | `Expenses/Expense Summaries` |
| 44 | `freshbooks expenses vendors` | `Expenses.Vendors` | RO | S | `Expenses/Expense Vendors` |
| 45 | `freshbooks expenses create-recurring` | `Expenses.CreateRecurring` | W | S, B | `Expenses/Create Recurring Expense` |
| 46 | `freshbooks gateways get` | `Gateways.Get` | RO | S, ID | `Tokenization/1a. [STRIPE] -  Get Publishable Key`; `Settings/Businesses/Gateway Details`; `Settings/Gateways/List Gateways` |
| 47 | `freshbooks identity me` | `Identity.Me` | RO | - | `Authorization/Identity Info Call`; `Authorization/List User` |
| 48 | `freshbooks identity whoami` | `Identity.Whoami` | RO | - | - |
| 49 | `freshbooks identity register` | `Identity.Register` | W | B | `Authorization/Register as a new user` |
| 50 | `freshbooks images upload` | `Images.Upload` | W | S, `--file <path>` | `Uploader/Upload Logo or Proposal Image`; `Invoices/Upload Logo/Upload Logo`; `Expenses/Upload Expense Receipt Image/Upload Receipt Image` |
| 51 | `freshbooks images upload-without-account` | `Images.UploadWithoutAccount` | W | S, `--file <path>` | `Uploader/Upload Image Without AccountId`; `Settings/Developer/Upload App Logo` |
| 52 | `freshbooks invoice-profiles list` | `InvoiceProfiles.List` | RO | S, L+all | `Invoices/Invoice Recurring Template/List Invoice Profiles` |
| 53 | `freshbooks invoice-profiles get` | `InvoiceProfiles.Get` | RO | S, ID | `Invoices/Invoice Recurring Template/Get Single Invoice Profile` |
| 54 | `freshbooks invoice-profiles create` | `InvoiceProfiles.Create` | W | S, B | `Invoices/Invoice Recurring Template/Create Single Invoice Profile`; `Invoices/Invoice Recurring Template/Create Single Invoice Profile w/ Time Entry Holder` |
| 55 | `freshbooks invoice-profiles update` | `InvoiceProfiles.Update` | I | S, ID, B | `Invoices/Invoice Recurring Template/Update Invoice Profile` |
| 56 | `freshbooks invoice-profiles delete` | `InvoiceProfiles.Delete` | D | S, ID | `Invoices/Invoice Recurring Template/Delete  Invoice Profile` |
| 57 | `freshbooks invoice-profiles enable-payment-options` | `InvoiceProfiles.EnablePaymentOptions` | I | S, ID, B | `Invoices/Invoice Recurring Template/Enable Payment Options On Invoice Profile` |
| 58 | `freshbooks invoices list` | `Invoices.List` | RO | S, L+all | `Invoices/List Invoices` |
| 59 | `freshbooks invoices get` | `Invoices.Get` | RO | S, ID | `Invoices/Single Invoice`; `Invoices/Single Invoice w/ Logo` |
| 60 | `freshbooks invoices create` | `Invoices.Create` | W | S, B | `Invoices/Create Invoice with Expense`; `Invoices/Single Invoice w/ Line Items`; `Invoices/Single Invoice w/ Logo and styles`; `Invoices/Single Invoice w/ Payment Gateway` |
| 61 | `freshbooks invoices update` | `Invoices.Update` | I | S, ID, B | `Invoices/Update Invoice`; `Invoices/Update Invoice w/ Expense`; `Invoices/Toggle Online Payments on Invoice` |
| 62 | `freshbooks invoices delete` | `Invoices.Delete` | D | S, ID | `Invoices/Delete  Invoice` |
| 63 | `freshbooks invoices send` | `Invoices.Send` | W | S, ID, B | `Invoices/Send Invoice by Email` |
| 64 | `freshbooks invoices pdf` | `Invoices.PDF` | RO | S, ID, `-o <file>` | `Invoices/Invoice Links/Downloads/Download Invoice PDF` |
| 65 | `freshbooks invoices share-link` | `Invoices.ShareLink` | RO | S, ID | `Invoices/Invoice Links/Downloads/Share Link`; `Invoices/Invoice Links/Downloads/Share PDF` |
| 66 | `freshbooks invoices enable-payment-options` | `Invoices.EnablePaymentOptions` | I | S, ID, B | `Invoices/Enable Payment Options On Invoice` |
| 67 | `freshbooks invoices invoice-presentation-defaults` | `Invoices.InvoicePresentationDefaults` | RO | S | `Invoices/Get default invoice presentation styles` |
| 68 | `freshbooks items list` | `Items.List` | RO | S, L+all | `Invoices/Items and Services/List Items`; `Invoices/Items and Services/List Items Filtered by SKU`; `Settings/Items and Services/List Items`; `Settings/Items and Services/List Items Filtered by SKU` |
| 69 | `freshbooks items get` | `Items.Get` | RO | S, ID | `Invoices/Items and Services/Single Item`; `Settings/Items and Services/Single Item` |
| 70 | `freshbooks items create` | `Items.Create` | W | S, B | `Invoices/Items and Services/Create Item`; `Settings/Items and Services/Create Item` |
| 71 | `freshbooks items update` | `Items.Update` | I | S, ID, B | `Invoices/Items and Services/Update Item`; `Settings/Items and Services/Update Item` |
| 72 | `freshbooks items delete` | `Items.Delete` | D | S, ID | `Invoices/Items and Services/Delete Item`; `Settings/Items and Services/Delete Item` |
| 73 | `freshbooks journal-entries create` | `JournalEntries.Create` | W | S, B | `Accounting/Journal Entries/Add Journal Entry` |
| 74 | `freshbooks journal-entries details` | `JournalEntries.Details` | RO | S | `Accounting/Journal Entries/Journal Entry Details` |
| 75 | `freshbooks journal-entry-accounts list` | `JournalEntryAccounts.List` | RO | S, L | `Accounting/Journal Entries/Accounts`; `Reports/General Ledger` |
| 76 | `freshbooks ledger-accounts create` | `LedgerAccounts.Create` | W | S, B | `Accounting/Accounts/Create Account` |
| 77 | `freshbooks ledger-accounts list` | `LedgerAccounts.List` | RO | S, L | `Accounting/Accounts/List Accounts` |
| 78 | `freshbooks ledger-accounts get` | `LedgerAccounts.Get` | RO | S, ID | `Accounting/Accounts/Single Account` |
| 79 | `freshbooks ledger-accounts update` | `LedgerAccounts.Update` | I | S, ID, B | `Accounting/Accounts/Update Account` |
| 80 | `freshbooks ledger-accounts types` | `LedgerAccounts.Types` | RO | S | `Accounting/Accounts/List Account types` |
| 81 | `freshbooks ledger-accounts sub-types` | `LedgerAccounts.SubTypes` | RO | S | `Accounting/Accounts/List Sub types` |
| 82 | `freshbooks ledger-accounts sub-type` | `LedgerAccounts.SubType` | RO | S, ID | `Accounting/Accounts/Single Sub type` |
| 83 | `freshbooks other-income create` | `OtherIncome.Create` | W | S, B | `Accounting/Other Income/Create Single Other Income`; `Invoices/Other Income/Create Single Other Income` |
| 84 | `freshbooks other-income list` | `OtherIncome.List` | RO | S, L+all | `Accounting/Other Income/List Other Income`; `Invoices/Other Income/List Other Income` |
| 85 | `freshbooks other-income update` | `OtherIncome.Update` | I | S, ID, B | `Accounting/Other Income/Update Single Other Income`; `Invoices/Other Income/Update Single Other Income` |
| 86 | `freshbooks other-income delete` | `OtherIncome.Delete` | D | S, ID | `Accounting/Other Income/Delete Single Other Income`; `Invoices/Other Income/Delete Single Other Income` |
| 87 | `freshbooks payment-options fb-pay-tokenize` | `PaymentOptions.FBPayTokenize` | W | S, B | `Tokenization/1. [FBPAY] - Create Payment Method` |
| 88 | `freshbooks payment-options stripe-tokenize` | `PaymentOptions.StripeTokenize` | W | S, B | `Tokenization/1. [STRIPE] - Create Payment Method` |
| 89 | `freshbooks payment-options stripe-create-setup-intent` | `PaymentOptions.StripeCreateSetupIntent` | W | S, `--rate`/`--payment-method` | `Tokenization/2. [STRIPE] - Create Setup Intent Using Payment Method Key` |
| 90 | `freshbooks payment-options save-credit-card` | `PaymentOptions.SaveCreditCard` | W | S, B | `Tokenization/2. [FBPAY] - Create Setup Intent Using Payment Method Key`; `Tokenization/3. [STRIPE] - Save Payment Method to Recurring Profile` |
| 91 | `freshbooks payments list` | `Payments.List` | RO | S, L+all | `Invoices/Payments/List Payments` |
| 92 | `freshbooks payments get` | `Payments.Get` | RO | S, ID | `Invoices/Payments/Single Payment` |
| 93 | `freshbooks payments create` | `Payments.Create` | W | S, B | `Invoices/Payments/Make Payment` |
| 94 | `freshbooks payments update` | `Payments.Update` | I | S, ID, B | `Invoices/Payments/Update Payment` |
| 95 | `freshbooks payments delete` | `Payments.Delete` | D | S, ID | `Invoices/Payments/Delete Payment` |
| 96 | `freshbooks payments create-checkout-link` | `Payments.CreateCheckoutLink` | W | S, B | `Invoices/Payments/Single Checkout Link` |
| 97 | `freshbooks payments update-checkout-link` | `Payments.UpdateCheckoutLink` | I | S, ID, B | `Invoices/Payments/Update Checkout Link` |
| 98 | `freshbooks payments delete-checkout-link` | `Payments.DeleteCheckoutLink` | D | S, ID | `Invoices/Payments/Delete Checkout Link` |
| 99 | `freshbooks payments update-checkout-link-gateway` | `Payments.UpdateCheckoutLinkGateway` | I | S, ID, B | `Invoices/Payments/Update Checkout Link Payment Gateway` |
| 100 | `freshbooks projects create` | `Projects.Create` | W | S, B | `Projects/Create Single Project` |
| 101 | `freshbooks projects get` | `Projects.Get` | RO | S, ID | `Projects/Single Project` |
| 102 | `freshbooks projects list` | `Projects.List` | RO | S, L+all | `Projects/List Projects` |
| 103 | `freshbooks projects update` | `Projects.Update` | I | S, ID, B | `Projects/Update Project` |
| 104 | `freshbooks projects delete` | `Projects.Delete` | D | S, ID | `Projects/Delete Project` |
| 105 | `freshbooks projects abilities` | `Projects.Abilities` | RO | S | `Settings/Abilities/Abilities` |
| 106 | `freshbooks projects threads` | `Projects.Threads` | RO | S, ID | `Projects/List All Messages in Project Discussion` |
| 107 | `freshbooks projects create-thread` | `Projects.CreateThread` | W | S, ID, `--message` | `Projects/Create New Message in Project Discussion` |
| 108 | `freshbooks projects add-thread-comment` | `Projects.AddThreadComment` | W | S, ID, `--message` | `Projects/Add Comment to Project Discussion Message` |
| 109 | `freshbooks reports accounts-aging` | `Reports.AccountsAging` | RO | S, report opts as flags | `Reports/Accounts Aging` |
| 110 | `freshbooks reports balance-sheet` | `Reports.BalanceSheet` | RO | S, report opts as flags | `Reports/Balance Sheet` |
| 111 | `freshbooks reports bank-reconciliation-summary` | `Reports.BankReconciliationSummary` | RO | S, report opts as flags | `Reports/Bank Reconciliation Summary` |
| 112 | `freshbooks reports client-account-statement` | `Reports.ClientAccountStatement` | RO | S, report opts as flags | `Reports/Client Account Statement` |
| 113 | `freshbooks reports download-invoice-details-csv` | `Reports.DownloadInvoiceDetailsCSV` | RO | S, `-o <file>`, `<download-token>` | `Reports/Download CSV Report` |
| 114 | `freshbooks reports expense-details` | `Reports.ExpenseDetails` | RO | S, report opts as flags | `Reports/Expense Details` |
| 115 | `freshbooks reports invoice-details` | `Reports.InvoiceDetails` | RO | S, report opts as flags | `Reports/Invoice Details` |
| 116 | `freshbooks reports item-sales` | `Reports.ItemSales` | RO | S, report opts as flags | `Reports/Item Sales` |
| 117 | `freshbooks reports payments-collected` | `Reports.PaymentsCollected` | RO | S, report opts as flags | `Reports/Payments Collected` |
| 118 | `freshbooks reports profit-loss` | `Reports.ProfitLoss` | RO | S, report opts as flags | `Reports/Profit/Loss Report` |
| 119 | `freshbooks reports revenue-by-client` | `Reports.RevenueByClient` | RO | S, report opts as flags | `Reports/Revenue By Client` |
| 120 | `freshbooks reports sales-tax-summary` | `Reports.SalesTaxSummary` | RO | S, report opts as flags | `Reports/Sales Tax Summary` |
| 121 | `freshbooks reports trial-balance` | `Reports.TrialBalance` | RO | S, report opts as flags | `Reports/Trial Balance` |
| 122 | `freshbooks reports time-entry-details` | `Reports.TimeEntryDetails` | RO | S | `Reports/Time Entry Details` |
| 123 | `freshbooks retainers list` | `Retainers.List` | RO | S, L | `Invoices/Retainers/Get all retainers` |
| 124 | `freshbooks retainers get` | `Retainers.Get` | RO | S, ID | `Invoices/Retainers/Single Retainer` |
| 125 | `freshbooks retainers create` | `Retainers.Create` | W | S, B | `Invoices/Retainers/Create Retainer` |
| 126 | `freshbooks retainers update` | `Retainers.Update` | I | S, ID, B | `Invoices/Retainers/Update Retainer` |
| 127 | `freshbooks retainers delete` | `Retainers.Delete` | D | S, ID | `Invoices/Retainers/Delete Retainer` |
| 128 | `freshbooks retainers undelete` | `Retainers.Undelete` | I | S, ID | `Invoices/Retainers/Undelete Retainer` |
| 129 | `freshbooks service-rates get` | `ServiceRates.Get` | RO | S, ID | `Settings/Items and Services/Get a Single Service Rate` |
| 130 | `freshbooks service-rates list` | `ServiceRates.List` | RO | S, L | `Projects/Service Rates` |
| 131 | `freshbooks service-rates update-project-rate` | `ServiceRates.UpdateProjectRate` | I | S, ID, `--rate`/`--payment-method` | `Projects/Update Service Rates` |
| 132 | `freshbooks services get` | `Services.Get` | RO | S, ID | `Settings/Items and Services/Get a Single Service` |
| 133 | `freshbooks services list` | `Services.List` | RO | S, L | `Settings/Items and Services/List Services` |
| 134 | `freshbooks services create` | `Services.Create` | W | S, B | `Settings/Items and Services/Create Service` |
| 135 | `freshbooks services get-billable-item` | `Services.GetBillableItem` | RO | S, ID | `Settings/Items and Services/Single Service` |
| 136 | `freshbooks identity add-business` | `Identity.AddBusiness` | W | B | `Settings/Businesses/Add Business` |
| 137 | `freshbooks identity delete-business` | `Identity.DeleteBusiness` | D | S | `Settings/Businesses/Delete Business` |
| 138 | `freshbooks identity delete-business-subscription` | `Identity.DeleteBusinessSubscription` | D | S | `Settings/Businesses/Delete Business - Subscription` |
| 139 | `freshbooks identity provision-payments` | `Identity.ProvisionPayments` | W | S, B | `Settings/Businesses/Provision FreshBooks Payments` |
| 140 | `freshbooks identity create-application` | `Identity.CreateApplication` | W | B | `Settings/Developer/Create new application` |
| 141 | `freshbooks identity applications` | `Identity.Applications` | RO | - | `Settings/Developer/Get all applications` |
| 142 | `freshbooks identity update-application` | `Identity.UpdateApplication` | I | ID, B | `Settings/Developer/Modify existing application` |
| 143 | `freshbooks staff list` | `Staff.List` | RO | S, L | `My Team/List Staff` |
| 144 | `freshbooks staff get` | `Staff.Get` | RO | S, ID | `My Team/Single Staff` |
| 145 | `freshbooks staff update` | `Staff.Update` | I | S, ID, B | `My Team/Update Staff` |
| 146 | `freshbooks staff delete` | `Staff.Delete` | D | S, ID | `My Team/Delete Staff` |
| 147 | `freshbooks systems get` | `Systems.Get` | RO | S, ID | `Settings/Systems/Get System` |
| 148 | `freshbooks tasks create` | `Tasks.Create` | W | S, B | `Projects/Tasks/Create Task` |
| 149 | `freshbooks tasks get` | `Tasks.Get` | RO | S, ID | `Projects/Tasks/Single Task` |
| 150 | `freshbooks tasks list` | `Tasks.List` | RO | S, L+all | `Projects/Tasks/List Tasks` |
| 151 | `freshbooks tasks update` | `Tasks.Update` | I | S, ID, B | `Projects/Tasks/Update Task` |
| 152 | `freshbooks tasks delete` | `Tasks.Delete` | D | S, ID | `Projects/Tasks/Delete Task` |
| 153 | `freshbooks taxes list` | `Taxes.List` | RO | S, L+all | `Expenses/List Taxes`; `Accounting/Taxes/List Taxes`; `Settings/Items and Services/List Taxes` |
| 154 | `freshbooks taxes get` | `Taxes.Get` | RO | S, ID | `Expenses/Single Tax (GET)`; `Accounting/Taxes/Get Single Tax`; `Settings/Items and Services/Single Tax (GET)` |
| 155 | `freshbooks taxes create` | `Taxes.Create` | W | S, B | `Expenses/Create Single Tax`; `Accounting/Taxes/Create Single Tax`; `Settings/Items and Services/Create Single Tax` |
| 156 | `freshbooks taxes update` | `Taxes.Update` | I | S, ID, B | `Expenses/Update Tax`; `Accounting/Taxes/Update Single Tax`; `Settings/Items and Services/Update Tax` |
| 157 | `freshbooks taxes delete` | `Taxes.Delete` | D | S, ID | `Expenses/Single Tax (DELETE)`; `Accounting/Taxes/Delete Single Tax`; `Settings/Items and Services/Single Tax (DELETE)` |
| 158 | `freshbooks team-members list` | `TeamMembers.List` | RO | S, L+all | `My Team/List Team Members` |
| 159 | `freshbooks team-members get` | `TeamMembers.Get` | RO | S, ID | `My Team/Single Team Member` |
| 160 | `freshbooks team-members invitation-rates` | `TeamMembers.InvitationRates` | RO | S | `Projects/Invitation Rates` |
| 161 | `freshbooks team-members rates` | `TeamMembers.Rates` | RO | S | `Projects/Team Member Rates` |
| 162 | `freshbooks team-members update-rate` | `TeamMembers.UpdateRate` | I | S, ID, `--rate`/`--payment-method` | `My Team/Update Staff Rates`; `Projects/Update Team Member Rate` |
| 163 | `freshbooks team-members invite` | `TeamMembers.Invite` | W | S, B | `Projects/Invite Team Member to Project(s)` |
| 164 | `freshbooks time-entries list` | `TimeEntries.List` | RO | S, L | `Time Tracking/List Entries`; `Time Tracking/Time Entries Updated Since Precise Time`; `Time Tracking/Time Entries for a Given Day` |
| 165 | `freshbooks time-entries search` | `TimeEntries.Search` | RO | S, L, `<query>` | `Time Tracking/Time Entries For Employee on Specific Project` |
| 166 | `freshbooks time-entries create` | `TimeEntries.Create` | W | S, B | `Time Tracking/Create a Time Entry` |
| 167 | `freshbooks time-entries update` | `TimeEntries.Update` | I | S, ID, B | `Time Tracking/Update a Time Entry` |
| 168 | `freshbooks time-entries delete` | `TimeEntries.Delete` | D | S, ID | `Time Tracking/Delete a Time Entry` |
| 169 | `freshbooks time-entries list-with-totals` | `TimeEntries.ListWithTotals` | RO | S, L | - |

**Totals:** 169 registry commands (169 lib methods, 212 inventory keys); `--all` covers the 17 `All` iterators (BillVendors, Bills, Callbacks, Clients, CreditNotes, Estimates, ExpenseCategories, Expenses, InvoiceProfiles, Invoices, Items, OtherIncome, Payments, Projects, Tasks, Taxes, TeamMembers). Row 169 (Phase 8 convergence, 2026-09-03) is keyless like `identity whoami`: it wraps the same wire endpoint as row 164 (`time-entries list`), not a distinct Postman request, so it carries none of that endpoint's three keys a second time.
