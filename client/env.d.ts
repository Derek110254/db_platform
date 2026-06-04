/// <reference types="vite/client" />

declare module '*.vue' {
    import type { DefineComponent } from 'vue'
    const component: DefineComponent<{}, {}, any>
    export default component
  }

declare module 'encryptlong' {
  export default class JSEncrypt {
    constructor();
    setPublicKey(pubkey: string): void;
    setPrivateKey(privkey: string): void;
    encryptLong(str: string): string | false;
    decryptLong(str: string): string | false;
  }
}