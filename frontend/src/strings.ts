/** UI text. The site is Portuguese only. */
export const strings = {
  home: {
    title: 'notas',
    tagline: 'Um bloco de notas colaborativo, sem login.',
    intro:
      'A URL é a nota. Abra notas.lauraesteves.com/qualquer-coisa, escreva, e quem estiver na mesma URL vê o texto mudar na hora. Salva sozinho. Subpáginas também: /faculdade/calculo.',
    openLabel: 'Abrir nota',
    openPlaceholder: 'lista-de-compras',
    openButton: 'Abrir',
    openHint: 'Letras, números, - e _. Use / para subpáginas (até 5 níveis).',
    warningTitle: 'Antes de escrever',
    warningPublic:
      'As notas são públicas: qualquer pessoa com a URL pode ler e editar. Não guarde senhas nem dados pessoais.',
    warningExpiry:
      'Uma nota é apagada 15 dias depois da última edição. Editou, o prazo recomeça.',
    curlTitle: 'Pelo terminal',
    curlRead: 'ler',
    curlWrite: 'escrever (sobrescreve)',
    curlPipe: 'ou mandar a saída de um comando',
    curlFile: 'arquivo.txt',
  },
  note: {
    editorLabel: 'Texto da nota',
    editorHint:
      'Tab insere uma tabulação. Esc e depois Tab para sair do campo.',
    loading: 'Carregando…',
  },
  status: {
    label: 'Estado da nota',
    loading: 'carregando…',
    saved: 'salvo',
    saving: 'salvando…',
    offline: 'offline',
    conflict: 'conflito',
    tooLarge: (max: string) => `grande demais (máx. ${max} bytes), não salva`,
    retrying: 'muitas edições, tentando de novo…',
    error: 'erro ao salvar, tentando de novo',
    people: (n: number) => (n === 1 ? '1 pessoa aqui' : `${n} pessoas aqui`),
    expiresOn: (date: string) => `apagada em ${date} se ninguém editar`,
  },
  conflict: {
    title: 'Alguém editou esta nota',
    description:
      'Chegou uma versão nova enquanto você tinha alterações não salvas.',
    useRemote: 'Usar a versão nova',
    overwrite: 'Sobrescrever com a minha',
  },
  invalid: {
    title: 'Essa nota não existe',
    reserved: (segment: string) => `“${segment}” é um nome reservado.`,
    tooDeep: (max: number) => `No máximo ${max} níveis de subpáginas.`,
    tooLong: (max: number) => `O caminho pode ter no máximo ${max} caracteres.`,
    invalidSegment:
      'Cada parte do caminho precisa ter de 1 a 64 letras, números, - ou _.',
    empty: 'Caminho vazio.',
    back: 'Voltar para o início',
  },
  menu: {
    open: 'Abrir menu',
    close: 'Fechar menu',
  },
  footer: {
    name: 'Laura Esteves',
    report: 'Denunciar uma nota',
    reportSubject: 'Denúncia de nota',
  },
  githubCorner: 'Ver o código no GitHub',
} as const;
