declare module 'mailparser' {
  export interface ParsedMailAddress {
    text?: string
  }

  export interface ParsedMail {
    messageId?: string
    from?: ParsedMailAddress
    to?: ParsedMailAddress | ParsedMailAddress[]
    subject?: string
    date?: Date
    text?: string
    html?: string | false
  }

  export function simpleParser(source: Buffer): Promise<ParsedMail>
}
