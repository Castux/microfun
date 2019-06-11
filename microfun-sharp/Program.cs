using System;
using System.Collections.Generic;

class Program
{
    static int Main(string[] args)
    {
        // Get arguments from command line

        if (args.Length < 1)
        {
            Console.WriteLine("Invalid arguments");
            PrintUsage();
            return 1;
        }

        // Lex files

        var tokens = new List<Token>();
        var success = true;

        foreach(var path in args)
        {
            success = LexFile(path, tokens) && success;
        }

        if (!success)
        {
            Console.WriteLine("Failure");
            return 1;
        }

        // End of stream token

        var lastTokenPos = tokens[tokens.Count - 1].Position;
        var eosPos = new SourcePosition(lastTokenPos.File, lastTokenPos.End + 1, lastTokenPos.End + 1);
        tokens.Add(new Token(Token.Kind.EOS, eosPos, null));

        // Parse stream of tokens

        var parser = new Parser(tokens);
        var expr = parser.Parse();

        if(expr == null)
        {
            parser.Report.Print();
            Console.WriteLine("Failure");
            return 0;
        }

        // Analysis

        var analyzer = new Analyzer(expr);
        success = analyzer.Analyze();

        if(!success)
        {
            analyzer.Report.Print();
            Console.WriteLine("Failure");
            return 0;
        }

        // Interpretation

        var interpreter = new Interpreter(expr);
        success = interpreter.Evaluate();
        if(!success)
        {
            interpreter.Report.Print();
            Console.WriteLine("Failure");
            return 0;
        }

        Console.WriteLine("Success");
        return 0;
    }

    private static bool LexFile(string path, List<Token> tokens)
    {
        // Load file

        var file = new SourceFile(path);
        var success = file.Load();

        if (!success)
        {
            Console.WriteLine("Could not load file " + path);
            return false;
        }

        // Lex file

        var lexer = new Lexer(file);
        success = lexer.Lex(tokens);

        if (!success)
        {
            lexer.Report.Print();
        }

        return success;
    }

    static void PrintUsage()
    {
        Console.WriteLine("usage: microfun source1.mf source2.mf ...");
    }
}