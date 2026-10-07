/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
// Stored identities remain E.164. Country prefixes are separate in editors.
export function nationalPhoneNumber(value: string): string {
  const phone = value.trim()
  return phone.replace(/^(?:\+86|0086|86)\s*(?=1[3-9]\d{9}$)/, '')
}

export function formatMobilePhone(value: string): string {
  const national = nationalPhoneNumber(value)
  return /^1[3-9]\d{9}$/.test(national)
    ? `+86 ${national.slice(0, 3)} ${national.slice(3, 7)} ${national.slice(7)}`
    : value.trim()
}

export function canonicalMobilePhone(value: string): string {
  const national = nationalPhoneNumber(value)
  return /^1[3-9]\d{9}$/.test(national) ? `+86${national}` : value.trim()
}
