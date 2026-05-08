import Contacts
import Foundation

struct ClawdexContact: Codable {
    let identifier: String
    let first_name: String
    let last_name: String
    let full_name: String
    let emails: [String]
    let phones: [String]
}

func fail(_ message: String) -> Never {
    FileHandle.standardError.write(Data((message + "\n").utf8))
    exit(1)
}

let store = CNContactStore()
let status = CNContactStore.authorizationStatus(for: .contacts)

switch status {
case .authorized:
    break
case .notDetermined:
    let sem = DispatchSemaphore(value: 0)
    var granted = false
    var requestError: Error?
    store.requestAccess(for: .contacts) { ok, err in
        granted = ok
        requestError = err
        sem.signal()
    }
    _ = sem.wait(timeout: .now() + 60)
    if !granted {
        if let requestError {
            fail("Contacts access denied: \(requestError.localizedDescription)")
        }
        fail("Contacts access denied. Grant access in System Settings > Privacy & Security > Contacts.")
    }
case .denied, .restricted:
    fail("Contacts access denied. Grant access in System Settings > Privacy & Security > Contacts.")
@unknown default:
    fail("Contacts access is unavailable for this process.")
}

let keys: [CNKeyDescriptor] = [
    CNContactIdentifierKey as CNKeyDescriptor,
    CNContactFormatter.descriptorForRequiredKeys(for: .fullName),
    CNContactOrganizationNameKey as CNKeyDescriptor,
    CNContactEmailAddressesKey as CNKeyDescriptor,
    CNContactPhoneNumbersKey as CNKeyDescriptor,
]

let request = CNContactFetchRequest(keysToFetch: keys)
let encoder = JSONEncoder()

do {
    try store.enumerateContacts(with: request) { contact, _ in
        let emails = contact.emailAddresses.map { String($0.value) }.filter { !$0.isEmpty }
        let phones = contact.phoneNumbers.map { $0.value.stringValue }.filter { !$0.isEmpty }
        guard !emails.isEmpty || !phones.isEmpty else { return }

        var fullName = CNContactFormatter.string(from: contact, style: .fullName) ?? ""
        if fullName.isEmpty {
            fullName = contact.organizationName
        }
        guard !fullName.isEmpty else { return }

        let row = ClawdexContact(
            identifier: contact.identifier,
            first_name: contact.givenName,
            last_name: contact.familyName,
            full_name: fullName,
            emails: emails,
            phones: phones
        )
        if let data = try? encoder.encode(row),
           let line = String(data: data, encoding: .utf8) {
            print(line)
        }
    }
} catch {
    fail("Failed to enumerate Contacts: \(error.localizedDescription)")
}
