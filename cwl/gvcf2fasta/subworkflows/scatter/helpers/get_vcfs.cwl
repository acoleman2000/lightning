class: Workflow
cwlVersion: v1.2

inputs:
    vcf_files: 
        type: File[]?
    transformed_vcfs: 
        type: File[]?
steps: []
outputs:
    vcfs: 
        type: File[]
        outputSource:
            - vcf_files
            - transformed_vcfs
        pickValue: first_non_null